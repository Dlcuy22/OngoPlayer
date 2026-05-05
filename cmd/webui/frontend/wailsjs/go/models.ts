export namespace main {
	
	export class TrackInfo {
	    path: string;
	    name: string;
	    index: number;
	
	    static createFrom(source: any = {}) {
	        return new TrackInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.index = source["index"];
	    }
	}

}

