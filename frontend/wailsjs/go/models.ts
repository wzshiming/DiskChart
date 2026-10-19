export namespace app {
	
	export class Crumb {
	    ID: number;
	    Name: string;
	
	    static createFrom(source: any = {}) {
	        return new Crumb(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	    }
	}
	export class DeleteResult {
	    ParentID: number;
	    RemovedSize: number;
	    RemovedFiles: number;
	    RemovedDirs: number;
	
	    static createFrom(source: any = {}) {
	        return new DeleteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ParentID = source["ParentID"];
	        this.RemovedSize = source["RemovedSize"];
	        this.RemovedFiles = source["RemovedFiles"];
	        this.RemovedDirs = source["RemovedDirs"];
	    }
	}
	export class Entry {
	    ID: number;
	    Name: string;
	    RelPath: string;
	    Size: number;
	    Apparent: number;
	    IsDir: boolean;
	    FileCount: number;
	    ModTime: number;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.RelPath = source["RelPath"];
	        this.Size = source["Size"];
	        this.Apparent = source["Apparent"];
	        this.IsDir = source["IsDir"];
	        this.FileCount = source["FileCount"];
	        this.ModTime = source["ModTime"];
	    }
	}
	export class TreeChangedEvent {
	    RemovedID: number;
	    ParentID: number;
	
	    static createFrom(source: any = {}) {
	        return new TreeChangedEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.RemovedID = source["RemovedID"];
	        this.ParentID = source["ParentID"];
	    }
	}
	export class Summary {
	    Size: number;
	    Files: number;
	    Dirs: number;
	    Errors: number;
	    DurationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Size = source["Size"];
	        this.Files = source["Files"];
	        this.Dirs = source["Dirs"];
	        this.Errors = source["Errors"];
	        this.DurationMs = source["DurationMs"];
	    }
	}
	export class FinishedEvent {
	    Status: string;
	    Error: string;
	    Root: string;
	    RootID: number;
	    Summary: Summary;
	
	    static createFrom(source: any = {}) {
	        return new FinishedEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Status = source["Status"];
	        this.Error = source["Error"];
	        this.Root = source["Root"];
	        this.RootID = source["RootID"];
	        this.Summary = this.convertValues(source["Summary"], Summary);
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
	export class Progress {
	    Dirs: number;
	    Files: number;
	    Bytes: number;
	    Errors: number;
	    CurrentPath: string;
	    ElapsedMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Progress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Dirs = source["Dirs"];
	        this.Files = source["Files"];
	        this.Bytes = source["Bytes"];
	        this.Errors = source["Errors"];
	        this.CurrentPath = source["CurrentPath"];
	        this.ElapsedMs = source["ElapsedMs"];
	    }
	}
	export class EventPayloads {
	    Progress: Progress;
	    Finished: FinishedEvent;
	    TreeChanged: TreeChangedEvent;
	
	    static createFrom(source: any = {}) {
	        return new EventPayloads(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Progress = this.convertValues(source["Progress"], Progress);
	        this.Finished = this.convertValues(source["Finished"], FinishedEvent);
	        this.TreeChanged = this.convertValues(source["TreeChanged"], TreeChangedEvent);
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
	export class ExtStat {
	    Ext: string;
	    Category: string;
	    Size: number;
	    Count: number;
	
	    static createFrom(source: any = {}) {
	        return new ExtStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Ext = source["Ext"];
	        this.Category = source["Category"];
	        this.Size = source["Size"];
	        this.Count = source["Count"];
	    }
	}
	export class ExtReport {
	    Items: ExtStat[];
	    Other: ExtStat;
	    Total: number;
	
	    static createFrom(source: any = {}) {
	        return new ExtReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Items = this.convertValues(source["Items"], ExtStat);
	        this.Other = this.convertValues(source["Other"], ExtStat);
	        this.Total = source["Total"];
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
	
	
	export class Info {
	    Version: string;
	    Platform: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Version = source["Version"];
	        this.Platform = source["Platform"];
	    }
	}
	
	export class ScanError {
	    Path: string;
	    Msg: string;
	
	    static createFrom(source: any = {}) {
	        return new ScanError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Path = source["Path"];
	        this.Msg = source["Msg"];
	    }
	}
	export class ScanOptions {
	    SameDevice: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScanOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.SameDevice = source["SameDevice"];
	    }
	}
	export class ScanState {
	    Status: string;
	    Root: string;
	    RootID: number;
	    Progress: Progress;
	    Summary: Summary;
	    Error: string;
	    TreeStale: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScanState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Status = source["Status"];
	        this.Root = source["Root"];
	        this.RootID = source["RootID"];
	        this.Progress = this.convertValues(source["Progress"], Progress);
	        this.Summary = this.convertValues(source["Summary"], Summary);
	        this.Error = source["Error"];
	        this.TreeStale = source["TreeStale"];
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
	
	
	export class View {
	    ID: number;
	    Name: string;
	    Size: number;
	    Apparent: number;
	    IsDir: boolean;
	    FileCount: number;
	    Aggregate: boolean;
	    Count: number;
	    Skip: number;
	    Children: View[];
	
	    static createFrom(source: any = {}) {
	        return new View(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.Size = source["Size"];
	        this.Apparent = source["Apparent"];
	        this.IsDir = source["IsDir"];
	        this.FileCount = source["FileCount"];
	        this.Aggregate = source["Aggregate"];
	        this.Count = source["Count"];
	        this.Skip = source["Skip"];
	        this.Children = this.convertValues(source["Children"], View);
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
	export class Volume {
	    Mountpoint: string;
	    Label: string;
	    FSType: string;
	    Total: number;
	    Used: number;
	    Free: number;
	
	    static createFrom(source: any = {}) {
	        return new Volume(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Mountpoint = source["Mountpoint"];
	        this.Label = source["Label"];
	        this.FSType = source["FSType"];
	        this.Total = source["Total"];
	        this.Used = source["Used"];
	        this.Free = source["Free"];
	    }
	}

}

