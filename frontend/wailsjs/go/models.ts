export namespace app {
	
	export class OptionDTO {
	    flag: string;
	    type: string;
	    default: string;
	    category: string;
	    summary: string;
	    description: string;
	    risk: string;
	    affectsData: boolean;
	    enum: string[];
	    governed: string;
	    kinds: string[];
	
	    static createFrom(source: any = {}) {
	        return new OptionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.flag = source["flag"];
	        this.type = source["type"];
	        this.default = source["default"];
	        this.category = source["category"];
	        this.summary = source["summary"];
	        this.description = source["description"];
	        this.risk = source["risk"];
	        this.affectsData = source["affectsData"];
	        this.enum = source["enum"];
	        this.governed = source["governed"];
	        this.kinds = source["kinds"];
	    }
	}
	export class CategoryDTO {
	    name: string;
	    options: OptionDTO[];
	
	    static createFrom(source: any = {}) {
	        return new CategoryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.options = this.convertValues(source["options"], OptionDTO);
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
	export class CatalogDTO {
	    rcloneVersion: string;
	    categories: CategoryDTO[];
	
	    static createFrom(source: any = {}) {
	        return new CatalogDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rcloneVersion = source["rcloneVersion"];
	        this.categories = this.convertValues(source["categories"], CategoryDTO);
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
	
	export class CeilingsDTO {
	    transfers: number;
	    checkers: number;
	    bwlimit: string;
	    tpslimit: number;
	
	    static createFrom(source: any = {}) {
	        return new CeilingsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.transfers = source["transfers"];
	        this.checkers = source["checkers"];
	        this.bwlimit = source["bwlimit"];
	        this.tpslimit = source["tpslimit"];
	    }
	}
	export class ClampDTO {
	    flag: string;
	    requested: string;
	    applied: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new ClampDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.flag = source["flag"];
	        this.requested = source["requested"];
	        this.applied = source["applied"];
	        this.reason = source["reason"];
	    }
	}
	export class EffectiveDTO {
	    flag: string;
	    value: string;
	    risk: string;
	    affectsData: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EffectiveDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.flag = source["flag"];
	        this.value = source["value"];
	        this.risk = source["risk"];
	        this.affectsData = source["affectsData"];
	    }
	}
	export class EndpointDTO {
	    remote: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new EndpointDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.remote = source["remote"];
	        this.path = source["path"];
	    }
	}
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
	export class FileProgressDTO {
	    name: string;
	    bytes: number;
	    size: number;
	    speed: number;
	    percentage: number;
	    etaSeconds?: number;
	
	    static createFrom(source: any = {}) {
	        return new FileProgressDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.bytes = source["bytes"];
	        this.size = source["size"];
	        this.speed = source["speed"];
	        this.percentage = source["percentage"];
	        this.etaSeconds = source["etaSeconds"];
	    }
	}
	export class ImpactDTO {
	    level: string;
	    flag: string;
	    title: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new ImpactDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.flag = source["flag"];
	        this.title = source["title"];
	        this.detail = source["detail"];
	    }
	}
	
	export class PreviewDTO {
	    kind: string;
	    resolvedSrc: string;
	    resolvedDst: string;
	    argv: string[];
	    command: string;
	    effective: EffectiveDTO[];
	    clamps: ClampDTO[];
	    impacts: ImpactDTO[];
	    riskLevel: string;
	    requiresAck: boolean;
	    error?: ErrorDTO;
	
	    static createFrom(source: any = {}) {
	        return new PreviewDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.resolvedSrc = source["resolvedSrc"];
	        this.resolvedDst = source["resolvedDst"];
	        this.argv = source["argv"];
	        this.command = source["command"];
	        this.effective = this.convertValues(source["effective"], EffectiveDTO);
	        this.clamps = this.convertValues(source["clamps"], ClampDTO);
	        this.impacts = this.convertValues(source["impacts"], ImpactDTO);
	        this.riskLevel = source["riskLevel"];
	        this.requiresAck = source["requiresAck"];
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
	export class PreviewRequest {
	    kind: string;
	    single: Record<string, string>;
	    multi: Record<string, Array<string>>;
	    ceilings: CeilingsDTO;
	    src: EndpointDTO;
	    dst: EndpointDTO;
	
	    static createFrom(source: any = {}) {
	        return new PreviewRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.single = source["single"];
	        this.multi = source["multi"];
	        this.ceilings = this.convertValues(source["ceilings"], CeilingsDTO);
	        this.src = this.convertValues(source["src"], EndpointDTO);
	        this.dst = this.convertValues(source["dst"], EndpointDTO);
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
	export class StatsEventDTO {
	    bytes: number;
	    totalBytes: number;
	    speed: number;
	    errors: number;
	    checks: number;
	    transfers: number;
	    etaSeconds?: number;
	    activeJobs: number;
	    transferring: FileProgressDTO[];
	
	    static createFrom(source: any = {}) {
	        return new StatsEventDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bytes = source["bytes"];
	        this.totalBytes = source["totalBytes"];
	        this.speed = source["speed"];
	        this.errors = source["errors"];
	        this.checks = source["checks"];
	        this.transfers = source["transfers"];
	        this.etaSeconds = source["etaSeconds"];
	        this.activeJobs = source["activeJobs"];
	        this.transferring = this.convertValues(source["transferring"], FileProgressDTO);
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

