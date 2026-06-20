export namespace app {
	
	export class ErrorDTO {
	    code: string;
	    message: string;
	    retryable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ErrorDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	        this.retryable = source["retryable"];
	    }
	}
	export class StatusDTO {
	    daemonRunning: boolean;
	    remotes: string[];
	    bytes: number;
	    speed: number;
	    transfers: number;
	    errorsCount: number;
	    error?: ErrorDTO;
	
	    static createFrom(source: any = {}) {
	        return new StatusDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.daemonRunning = source["daemonRunning"];
	        this.remotes = source["remotes"];
	        this.bytes = source["bytes"];
	        this.speed = source["speed"];
	        this.transfers = source["transfers"];
	        this.errorsCount = source["errorsCount"];
	        this.error = this.convertValues(source["error"], ErrorDTO);
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

