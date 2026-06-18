export namespace main {
	
	export class TrackInfo {
	    path: string;
	    name: string;
	    index: number;
	    title: string;
	    artist: string;
	    album: string;
	    format: string;
	    hasCover: boolean;
	
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
	    }
	}

}

