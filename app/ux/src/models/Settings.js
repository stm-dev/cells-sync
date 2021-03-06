/**
 * Copyright 2019 Abstrium SAS
 *
 *  This file is part of Cells Sync.
 *
 *  Cells Sync is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  Cells Sync is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License
 *  along with Cells Sync.  If not, see <https://www.gnu.org/licenses/>.
 */
import buildUrl from './Url'

let listeners = [];

// Declare keys for the sake of auto-completion
class Settings {

    Logs = {
        Folder: "",
        MaxFilesNumber: 1,
        MaxFilesSize: 30,
        MaxAgeDays: 30
    };
    Updates = {
        Frequency: "restart",
        DownloadAuto: true,
        UpdateChannel: "",
        UpdateUrl: "",
        UpdatePublicKey: ""
    };
    Debugging = {
        ShowPanels: false
    };
    Service =  {
        AutoStart: false,
    };

    constructor(data) {
        if (data && data.Logs) {
            this.Logs = data.Logs;
        }
        if (data && data.Updates) {
            this.Updates = data.Updates;
        }
        if (data && data.Debugging){
            this.Debugging = data.Debugging;
        }
        if (data && data.Service){
            this.Service = data.Service;
        }
    }

    parseResponse(prom) {
        return prom.then(response => {
            if (response.status !== 200) {
                console.log(response);
                return response.json().then(data => {
                    console.log(data);
                    if(data && data.error) {
                        throw new Error(data.error);
                    }
                });
            }
            return response.json();
        }).then(data => {
            this.Logs = data.Logs;
            this.Updates = data.Updates;
            this.Debugging = data.Debugging || {};
            this.Service = data.Service || {};
            Settings.notify(this);
            return this;
        });
    }

    load(){
        return this.parseResponse(window.fetch(buildUrl('/config'), {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json'
            },
            credentials: 'omit'
        }));
    }

    save(){
        return this.parseResponse(window.fetch(buildUrl('/config'), {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            credentials: 'omit',
            body: JSON.stringify(this)
        }));
    }

    static observe(listener){
        listeners.push(listener)
    }

    static stopObserving(listener){
        listeners = listeners.filter((l) => l !== listener)
    }

    static notify(data){
        listeners.forEach(l => l(data));
    }

}

export default Settings;