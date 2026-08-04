export namespace backend {
	
	export class Device {
	    name: string;
	    host: string;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.host = source["host"];
	        this.model = source["model"];
	    }
	}
	export class Settings {
	    model: string;
	    host: string;
	    devices: Device[];
	    activeHost: string;
	    source: string;
	    resolution: number;
	    mode: string;
	    regionFull: boolean;
	    tlX: number;
	    tlY: number;
	    brX: number;
	    brY: number;
	    brightness: number;
	    contrast: number;
	    threshold: number;
	    format: string;
	    quality: number;
	    maxPages: number;
	    outputDir: string;
	    outputBase: string;
	    verbose: boolean;
	    language: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.host = source["host"];
	        this.devices = this.convertValues(source["devices"], Device);
	        this.activeHost = source["activeHost"];
	        this.source = source["source"];
	        this.resolution = source["resolution"];
	        this.mode = source["mode"];
	        this.regionFull = source["regionFull"];
	        this.tlX = source["tlX"];
	        this.tlY = source["tlY"];
	        this.brX = source["brX"];
	        this.brY = source["brY"];
	        this.brightness = source["brightness"];
	        this.contrast = source["contrast"];
	        this.threshold = source["threshold"];
	        this.format = source["format"];
	        this.quality = source["quality"];
	        this.maxPages = source["maxPages"];
	        this.outputDir = source["outputDir"];
	        this.outputBase = source["outputBase"];
	        this.verbose = source["verbose"];
	        this.language = source["language"];
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

