export namespace model {
	
	export class Song {
	    Name: string;
	    Path: string;
	    ObjectID: number;
	    StorageID: number;
	    Storage: string;
	
	    static createFrom(source: any = {}) {
	        return new Song(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Path = source["Path"];
	        this.ObjectID = source["ObjectID"];
	        this.StorageID = source["StorageID"];
	        this.Storage = source["Storage"];
	    }
	}

}

