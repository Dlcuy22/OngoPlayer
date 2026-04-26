// Package discordrpc provides Discord Rich Presence integration for the application,
// managing the IPC connection lifecycle and synchronizing the playback state
// with the user's Discord profile.
//
// Key Components:
//   - Manager: Owns the IPC connection, image upload process, and synchronization loop
//   - TrackInfo: Represents the snapshot of playback metadata passed from the UI
//
// Dependencies:
//   - github.com/axrona/go-discordrpc/client: Discord RPC IPC communication
//   - sync: Manages concurrent access to the metadata state
//   - image: Represents the raw cover art to be temporarily uploaded
//
// Error Types:
//   - Connection errors are handled internally by the retry loop and logged.
//
// Example:
//   rpc := discordrpc.New()
//   rpc.GetPosition = func() float64 { return engine.GetPosition() }
//   rpc.IsPaused = func() bool { return engine.GetState() == AudioEngine.StatePaused }
//   rpc.Start()
//   rpc.Update(discordrpc.TrackInfo{Title: "Song", DurationSec: 240, ElapsedSec: 10})
package discordrpc

import (
	"image"
	"log"
	"sync"
	"time"

	"github.com/axrona/go-discordrpc/client"
)

const (
	AppID         = "1498082439925334108"
	retryInterval = 30 * time.Second
	syncInterval  = 5 * time.Second
)

type TrackInfo struct {
	Title       string
	Artist      string
	Album       string
	Cover       image.Image // nil falls back to a default asset key
	DurationSec float64     // total track duration in seconds (0 = unknown)
	ElapsedSec  float64     // current playback position in seconds
	IsPaused    bool        // whether the track is currently paused
}

type Manager struct {
	uploader    *ImageUploader
	discord     *client.Client
	stopCh      chan struct{}
	updateCh    chan TrackInfo
	mu          sync.Mutex
	connected   bool
	current     TrackInfo
	startedAt   time.Time
	GetPosition func() float64 // callback to poll current playback position from the engine
	IsPaused    func() bool    // callback to poll current playback state from the engine
}

func New() *Manager {
	return &Manager{
		uploader: NewImageUploader(),
		discord:  client.NewClient(AppID),
		stopCh:   make(chan struct{}),
		updateCh: make(chan TrackInfo, 1),
	}
}

func (m *Manager) Start() {
	go m.loop()
}

func (m *Manager) Stop() {
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}
	m.disconnect()
}

func (m *Manager) Update(track TrackInfo) {
	// Non-blocking send: if the channel already has a pending update,
	// discard the stale one and replace with the newer payload.
	select {
	case m.updateCh <- track:
	default:
		<-m.updateCh
		m.updateCh <- track
	}
}

func (m *Manager) loop() {
	if err := m.connect(); err != nil {
		log.Printf("[discordrpc] initial connect failed: %v, will retry", err)
	}

	syncTicker := time.NewTicker(syncInterval)
	defer syncTicker.Stop()

	retryTicker := time.NewTicker(retryInterval)
	defer retryTicker.Stop()

	for {
		select {
		case <-m.stopCh:
			return

		case track := <-m.updateCh:
			m.mu.Lock()
			m.current = track
			m.startedAt = time.Now()
			m.mu.Unlock()

			if m.connected {
				m.setActivity(track)
			}

		case <-syncTicker.C:
			// Poll current position from the engine and re-send activity.
			// This keeps the progress bar in sync and handles seeks.
			if !m.connected || m.GetPosition == nil {
				continue
			}
			m.mu.Lock()
			track := m.current
			m.mu.Unlock()
			if track.Title == "" {
				continue
			}
			track.ElapsedSec = m.GetPosition()
			if m.IsPaused != nil {
				track.IsPaused = m.IsPaused()
			}
			m.setActivity(track)

		case <-retryTicker.C:
			if !m.connected {
				if err := m.connect(); err != nil {
					log.Printf("[discordrpc] reconnect failed: %v", err)
				} else {
					m.mu.Lock()
					track := m.current
					m.mu.Unlock()
					if track.Title != "" {
						m.setActivity(track)
					}
				}
			}
		}
	}
}

func (m *Manager) connect() error {
	if err := m.discord.Login(); err != nil {
		return err
	}
	m.connected = true
	log.Println("[discordrpc] connected to Discord")
	return nil
}

func (m *Manager) disconnect() {
	m.discord.Logout()
	m.connected = false
}

func (m *Manager) setActivity(track TrackInfo) {

	largeImage := "ongoplayer"
	largeText := "OngoPlayer"

	if track.Cover != nil {
		url, err := m.uploader.GetImageURL(track.Cover)
		if err != nil {
			log.Printf("[discordrpc] image upload failed: %v", err)
		} else {
			largeImage = url
			if track.Album != "" {
				largeText = track.Album
			}
		}
	}

	details := track.Title
	state := track.Artist
	if state == "" {
		state = "Unknown artist"
	}

	// Handle paused state or calculate timestamps for active playback
	var timestamps *client.Timestamps

	if track.IsPaused {
		largeText = "⏸ Paused"
	} else if track.DurationSec > 0 {
		now := time.Now()
		songStart := now.Add(-time.Duration(track.ElapsedSec * float64(time.Second)))
		songEnd := songStart.Add(time.Duration(track.DurationSec * float64(time.Second)))
		timestamps = &client.Timestamps{
			Start: &songStart,
			End:   &songEnd,
		}
	} else {
		now := time.Now()
		timestamps = &client.Timestamps{
			Start: &now,
		}
	}

	activity := client.Activity{
		Type:       2, // 0=Playing, 1=Streaming, 2=Listening, 3=Watching
		Details:    details,
		State:      state,
		LargeImage: largeImage,
		LargeText:  largeText,
		SmallImage: "ongoplayer_small",
		SmallText:  "OngoPlayer",
		Timestamps: timestamps,
	}

	log.Printf("[discordrpc] setActivity: details=%q state=%q largeImage=%q", details, state, largeImage)

	if err := m.discord.SetActivity(activity); err != nil {
		log.Printf("[discordrpc] SetActivity failed: %v", err)
		m.connected = false
	} else {
		log.Println("[discordrpc] SetActivity success")
	}
}
