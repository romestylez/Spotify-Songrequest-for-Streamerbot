export namespace main {
	
	export class SafeAppConfig {
	    client_id: string;
	    secret_set: boolean;
	    connected: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SafeAppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.client_id = source["client_id"];
	        this.secret_set = source["secret_set"];
	        this.connected = source["connected"];
	    }
	}
	export class SafeConfig {
	    server_port: number;
	    spotify_main: SafeAppConfig;
	    spotify_autoclear: SafeAppConfig;
	    playlist_id: string;
	    fav_playlist_id: string;
	
	    static createFrom(source: any = {}) {
	        return new SafeConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_port = source["server_port"];
	        this.spotify_main = this.convertValues(source["spotify_main"], SafeAppConfig);
	        this.spotify_autoclear = this.convertValues(source["spotify_autoclear"], SafeAppConfig);
	        this.playlist_id = source["playlist_id"];
	        this.fav_playlist_id = source["fav_playlist_id"];
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
	export class SaveInput {
	    server_port: number;
	    main_client_id: string;
	    main_client_secret: string;
	    ac_client_id: string;
	    ac_client_secret: string;
	    playlist_id: string;
	    fav_playlist_id: string;
	
	    static createFrom(source: any = {}) {
	        return new SaveInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_port = source["server_port"];
	        this.main_client_id = source["main_client_id"];
	        this.main_client_secret = source["main_client_secret"];
	        this.ac_client_id = source["ac_client_id"];
	        this.ac_client_secret = source["ac_client_secret"];
	        this.playlist_id = source["playlist_id"];
	        this.fav_playlist_id = source["fav_playlist_id"];
	    }
	}

}

