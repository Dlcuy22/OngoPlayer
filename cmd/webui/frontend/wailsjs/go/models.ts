export namespace main {
	
	export class AppConfig {
	    streamQuality: string;
	    streamCodec: string;
	    rpcEnabled: boolean;
	    volume: number;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.streamQuality = source["streamQuality"];
	        this.streamCodec = source["streamCodec"];
	        this.rpcEnabled = source["rpcEnabled"];
	        this.volume = source["volume"];
	    }
	}
	export class SearchContinuationResult {
	    items: any[];
	    nextToken?: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchContinuationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = source["items"];
	        this.nextToken = source["nextToken"];
	    }
	}
	export class TrackInfo {
	    path: string;
	    name: string;
	    index: number;
	    title: string;
	    artist: string;
	    album: string;
	    format: string;
	    hasCover: boolean;
	    lyricsBrowseID?: string;
	    ytmSongID?: string;
	    coverURL?: string;
	
	    static createFrom(source: any = {}) {
	        return new TrackInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.index = source["index"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.album = source["album"];
	        this.format = source["format"];
	        this.hasCover = source["hasCover"];
	        this.lyricsBrowseID = source["lyricsBrowseID"];
	        this.ytmSongID = source["ytmSongID"];
	        this.coverURL = source["coverURL"];
	    }
	}

}

export namespace ytm {
	
	export class PageRef {
	    browse_id: string;
	    browse_params?: string;
	
	    static createFrom(source: any = {}) {
	        return new PageRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.browse_id = source["browse_id"];
	        this.browse_params = source["browse_params"];
	    }
	}
	export class ArtistLayout {
	    items?: any[];
	    title?: string;
	    subtitle?: string;
	    type?: string;
	    view_more?: PageRef;
	    playlist_id?: string;
	
	    static createFrom(source: any = {}) {
	        return new ArtistLayout(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = source["items"];
	        this.title = source["title"];
	        this.subtitle = source["subtitle"];
	        this.type = source["type"];
	        this.view_more = this.convertValues(source["view_more"], PageRef);
	        this.playlist_id = source["playlist_id"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ThumbnailProvider {
	    url_a: string;
	    url_b?: string;
	
	    static createFrom(source: any = {}) {
	        return new ThumbnailProvider(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url_a = source["url_a"];
	        this.url_b = source["url_b"];
	    }
	}
	export class Artist {
	    id: string;
	    type?: string;
	    name?: string;
	    description?: string;
	    thumbnail?: ThumbnailProvider;
	    shuffle_playlist_id?: string;
	    layouts?: ArtistLayout[];
	    subscribe_channel_id?: string;
	    subscriber_count?: number;
	    subscribed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Artist(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.thumbnail = this.convertValues(source["thumbnail"], ThumbnailProvider);
	        this.shuffle_playlist_id = source["shuffle_playlist_id"];
	        this.layouts = this.convertValues(source["layouts"], ArtistLayout);
	        this.subscribe_channel_id = source["subscribe_channel_id"];
	        this.subscriber_count = source["subscriber_count"];
	        this.subscribed = source["subscribed"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class BuiltInContinuation {
	    token: string;
	    type: string;
	    item_id?: string;
	    playlist_skip_amount: number;
	
	    static createFrom(source: any = {}) {
	        return new BuiltInContinuation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.type = source["type"];
	        this.item_id = source["item_id"];
	        this.playlist_skip_amount = source["playlist_skip_amount"];
	    }
	}
	export class Chip {
	    name: string;
	    params?: string;
	    type?: string;
	
	    static createFrom(source: any = {}) {
	        return new Chip(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.params = source["params"];
	        this.type = source["type"];
	    }
	}
	export class MediaItemLayout {
	    items: any[];
	    title?: string;
	    subtitle?: string;
	    view_more?: PageRef;
	    type?: string;
	
	    static createFrom(source: any = {}) {
	        return new MediaItemLayout(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = source["items"];
	        this.title = source["title"];
	        this.subtitle = source["subtitle"];
	        this.view_more = this.convertValues(source["view_more"], PageRef);
	        this.type = source["type"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Song {
	    id: string;
	    name?: string;
	    description?: string;
	    thumbnail?: ThumbnailProvider;
	    artists?: Artist[];
	    type?: string;
	    is_explicit: boolean;
	    album?: Playlist;
	    duration_ms?: number;
	    related_browse_id?: string;
	    lyrics_browse_id?: string;
	
	    static createFrom(source: any = {}) {
	        return new Song(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.thumbnail = this.convertValues(source["thumbnail"], ThumbnailProvider);
	        this.artists = this.convertValues(source["artists"], Artist);
	        this.type = source["type"];
	        this.is_explicit = source["is_explicit"];
	        this.album = this.convertValues(source["album"], Playlist);
	        this.duration_ms = source["duration_ms"];
	        this.related_browse_id = source["related_browse_id"];
	        this.lyrics_browse_id = source["lyrics_browse_id"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Playlist {
	    id: string;
	    name?: string;
	    description?: string;
	    thumbnail?: ThumbnailProvider;
	    type?: string;
	    artists?: Artist[];
	    year?: number;
	    items?: Song[];
	    owner_id?: string;
	    continuation?: BuiltInContinuation;
	    item_set_ids?: string[];
	    item_count?: number;
	    total_duration_ms?: number;
	    playlist_url?: string;
	
	    static createFrom(source: any = {}) {
	        return new Playlist(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.thumbnail = this.convertValues(source["thumbnail"], ThumbnailProvider);
	        this.type = source["type"];
	        this.artists = this.convertValues(source["artists"], Artist);
	        this.year = source["year"];
	        this.items = this.convertValues(source["items"], Song);
	        this.owner_id = source["owner_id"];
	        this.continuation = this.convertValues(source["continuation"], BuiltInContinuation);
	        this.item_set_ids = source["item_set_ids"];
	        this.item_count = source["item_count"];
	        this.total_duration_ms = source["total_duration_ms"];
	        this.playlist_url = source["playlist_url"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SearchFilter {
	    type: string;
	    params: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.params = source["params"];
	    }
	}
	export class SearchCategory {
	    layout: MediaItemLayout;
	    filter?: SearchFilter;
	    continuation?: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchCategory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.layout = this.convertValues(source["layout"], MediaItemLayout);
	        this.filter = this.convertValues(source["filter"], SearchFilter);
	        this.continuation = source["continuation"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class SearchResults {
	    categories: SearchCategory[];
	    suggested_correction?: string;
	    chips?: Chip[];
	
	    static createFrom(source: any = {}) {
	        return new SearchResults(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.categories = this.convertValues(source["categories"], SearchCategory);
	        this.suggested_correction = source["suggested_correction"];
	        this.chips = this.convertValues(source["chips"], Chip);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

