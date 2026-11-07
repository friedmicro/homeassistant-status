# homeassistant-status

A simple service that fetches a select number of entities from home assistant and returns them in an API to be used by external automations. This repo includes a simple fetch service that will return the state of a list of entities from home assistant as well as an example of service which returns the respective data formatted\processed back to a user. Additionally, there is an example for waybar users.

Full disclosure, only the fetch service here is 100% usable to someone running home assistant. The waybar and api services are largely examples of how you can use the data returned from it. While they are fully working programs they reference entities that I have in my environment that you likely do not.

## Why?

You may ask why not just inferface with the home assistant API directly and while this "technically" works it has a lot of issues. For example there is no such thing as a read only token in home assistant and rather they gain all the permissions of the caller. So if you want to say expose a read only API for something like another service that may or may not be written by you then that is a massive issue. Additionally, the home assistant API requires you to know exact specifics about things like your entities and it can be fairly cumbersome to manage all that logic which is why I have a seperate service for processing this information.

Additionally, home assistant if deployed like most people do isn't a high availability deployment meaning that for updates (which are largely manual through the application and can even be required for certain config changes) you effectively take out the system for a few minutes...this is...very much suboptimal. As an aside this is also one of the reasons the automation state service https://github.com/friedmicro/automation-state and the kindle dashboard proxy exist https://github.com/friedmicro/kindle-dashboard-cache-proxy. Both of these allow states to be accessed when home assistant is down and can cache the result if needed.

To further complicate things, while home assistant is great at simple to moderate complexity integrations really complex automations can be tedious to implement and returning the state is not something that can be trivially done from inside the UI. Candidly this is actually one of the weaknesses the old automation state had because it was dependent upon home assistant correctly pushing the state and this was done through the RESTful Command service https://www.home-assistant.io/integrations/rest_command/. This service requires everything to be configured via yaml and makes defining a return state for an entity have to in yaml.

As to why there's an additional service...because home assistant can have very strange entity names and often a lot of keys you do not want there needs to be a layer to reduce the noise returned on the API while also simplifying things.

Some very real world\tangible examples of my issues currently that can't be cleanly solved within home assistant:
* I often forget to turn off my PC that I use for offloading compute heavy builds, streaming games, or AI workloads
* I also ocassionally have had my VR base stations fail to turn on through home assistant, if the status is reported unknown then I know there is an error and my VM watchdog can go through and restart the home assistant VM so that the base stations are connected before I need them
* I have historically throughout my life had a very broken sleep schedule so I need a way to render what time of day it is as well as when something like sunrise is. This tangible reminder allows me to go sleep when I should
* I regularly forget to charge my phone...a simple notification on my PC for low battery allows me to immediately know and take care of
* And sometimes...people just forget they left on the electric kettle in the kitchen so I need a notification for that
* I want an AI agent to report back information about the home status when I request it so that I know temperatures in rooms, co2 levels, etc without opening an app

The reason home assistant doesn't work is either the information is used for self-healing on the HA VM or what I need is to display information cleanly to myself in a readonly manner.

## How does this work?

This works by obtaining an access token from home assistant and using that on a service whose entire purpose is just to fetch entities and because the service is read only it does not pose a massive risk to modifying the home assistant data which means it can be used for other purposes. A secondary service (in this case API) does the actual processing\forming of the data so that a VM watchdog process can do recovery or so it can be rendered on my desktop so that I get the information I need.

End of the day this is really just a wrapper around home assistant so that you can request information from an API that is truly readonly and a few examples of it in use.

## Build\use
To build any of the serverside services either use the respective dockerfile (modifying the image tags to ones that are suitable to you) or run `go build .` in the respective directories.

For the waybar directory run `cargo builds --release` and copy the built binary into your location of your choice. Do note it depends upon this project to work over a local unix socket: https://github.com/friedmicro/waybar-net-daemon.

## Fetch service configuration

| Env var     | Required | Default | Meaning                                          |
|-------------|----------|---------|--------------------------------------------------|
| `HA_URL`    | no       | `http://serpent.friedmicro-lab.org:8123` | HA base URL |
| `HA_TOKEN`  | yes      | —       | HA long-lived access token             |
| `ENTITIES`  | yes      | —       | Entity list |
| `PORT`      | no       | `8080`  | Listen port                                     |

`ENTITIES` accepts a comma seperated list of entity names matching those in home assistant

The endpoint `/entities` returns the entities states. It is highly likely if you are building a custom solution with this that this is the only service you can use without modification.

## API service configuration

| Env var   | Required | Default | Meaning                                   |
|-----------|----------|---------|-------------------------------------------|
| `FETCH_URL` | no    | `http://homeassistant-status-fetch:8080` | service URL |
| `PORT`    | no       | `8080`  | Listen port                               |

The endpoint of node you want to call is `/status` which assuming your entities are the exact same as mine would return this.

```jsonc
{
  "raw":   { "<entity_id>": { "state": "...", "attributes": {} } }, // raw
  "computed": {
    "base_stations_on": false,
    "den_temperature": 77.82,
    "den_temperature_unit": "°F",
    "flags": {
      "base_station_service_offline": false,
      "lucia_phone_battery_low": false
    },
    "kettle_on": false,
    "lucia_phone": {
      "battery_level": "66",
      "charge_state": "discharging"
    },
    "offloaded_compute_on": false,
    "sun": {
      "next_event_in_hours": 3.3,
      "next_state": "dawn",
      "state": "dusk"
    }
  }
}
```
