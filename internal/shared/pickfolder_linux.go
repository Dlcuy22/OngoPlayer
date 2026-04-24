//go:build linux

// internal/shared/pickfolder_linux.go
// Linux folder picker using XDG Desktop Portal via D-Bus.
// Works across all major DEs (GNOME, KDE, XFCE, Sway, etc.)
// by talking to org.freedesktop.portal.FileChooser.
//
// Dependencies:
//   - github.com/godbus/dbus/v5: D-Bus session bus connection
//
// Requirements:
//   - xdg-desktop-portal + a backend (xdg-desktop-portal-gnome, -kde, etc.)

package shared

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	dbus "github.com/godbus/dbus/v5"
)

const (
	portalBus   = "org.freedesktop.portal.Desktop"
	portalPath  = "/org/freedesktop/portal/desktop"
	portalIface = "org.freedesktop.portal.FileChooser"
	reqIface    = "org.freedesktop.portal.Request"
)

/*
PickFolder opens a native folder picker dialog via XDG Desktop Portal.

	returns:
	      string: selected folder path
	      error:  on D-Bus failure or user cancellation
*/
func PickFolder() (string, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return "", fmt.Errorf("dbus: %w", err)
	}
	defer conn.Close()

	handleToken := fmt.Sprintf("dongoplayer_%d", time.Now().UnixNano())

	sender := conn.Names()[0]
	senderPath := strings.ReplaceAll(sender, ".", "_")
	senderPath = strings.ReplaceAll(senderPath, ":", "")

	responsePath := dbus.ObjectPath(fmt.Sprintf(
		"/org/freedesktop/portal/desktop/request/%s/%s",
		senderPath, handleToken,
	))

	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(responsePath),
		dbus.WithMatchInterface(reqIface),
		dbus.WithMatchMember("Response"),
	); err != nil {
		return "", fmt.Errorf("dbus signal match: %w", err)
	}

	signalCh := make(chan *dbus.Signal, 1)
	conn.Signal(signalCh)

	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(handleToken),
		"directory":    dbus.MakeVariant(true),
		"modal":        dbus.MakeVariant(true),
	}

	obj := conn.Object(portalBus, portalPath)
	call := obj.Call(portalIface+".OpenFile", 0, "", "Select Music Folder", options)
	if call.Err != nil {
		return "", fmt.Errorf("OpenFile: %w", call.Err)
	}

	select {
	case sig := <-signalCh:
		if len(sig.Body) < 2 {
			return "", fmt.Errorf("invalid response signal")
		}

		responseCode := sig.Body[0].(uint32)
		if responseCode != 0 {
			return "", fmt.Errorf("cancelled")
		}

		results := sig.Body[1].(map[string]dbus.Variant)
		uris := results["uris"].Value().([]string)

		if len(uris) == 0 {
			return "", fmt.Errorf("no folder selected")
		}

		return uriToPath(uris[0])

	case <-time.After(2 * time.Minute):
		return "", fmt.Errorf("timeout waiting for folder selection")
	}
}

func uriToPath(rawURI string) (string, error) {
	u, err := url.Parse(rawURI)
	if err != nil {
		return "", err
	}
	if u.Scheme == "file" {
		return u.Path, nil
	}
	return rawURI, nil
}
