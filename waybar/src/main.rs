use serde::Deserialize;
use std::io::{BufRead, BufReader, Read, Write};
use std::os::unix::net::UnixStream;

const DAEMON_SOCKET: &str = "/tmp/waybar-damon";
const DAEMON_KEY: &str = "ha";

#[derive(Deserialize)]
struct StatusResponse {
    computed: Computed,
}

#[derive(Deserialize)]
struct Computed {
    bedroom_window_open: bool,
    den_temperature: f64,
    den_temperature_unit: String,
    base_stations_on: bool,
    kettle_on: bool,
    offloaded_compute_on: bool,
    sun: Sun,
    flags: Flags,
}

#[derive(Deserialize)]
struct Sun {
    next_event_in_hours: f64,
    state: String,
}

#[derive(Deserialize)]
struct Flags {
    base_station_service_offline: bool,
    lucia_phone_battery_low: bool,
}

#[derive(serde::Serialize)]
struct Output {
    text: String,
    tooltip: String,
    class: String,
}

struct ToolbarSymbols {
    dawn: &'static str,
    rising: &'static str,
    noon: &'static str,
    setting: &'static str,
    dusk: &'static str,
    cell: String,
    base_station: String,
    kettle: String,
    window: String,
    offload_compute: String,
    temp: String,
}

fn get_symbols() -> ToolbarSymbols {
    return ToolbarSymbols {
        dawn: "🌿",
        rising: "🌄",
        noon: "🌞",
        setting: "🌇",
        dusk: "🌆",
        cell: String::from("📱"),
        base_station: String::from("🛙"),
        kettle: String::from("🫖"),
        window: String::from("🪟"),
        offload_compute: String::from("⚡"),
        temp: String::from("🌡️"),
    };
}

fn request_key() -> Result<String, String> {
    let mut conn =
        UnixStream::connect(&DAEMON_SOCKET).map_err(|e| format!("connect to daemon: {e}"))?;
    conn.write_all(format!("{DAEMON_KEY}\n").as_bytes())
        .map_err(|e| format!("write request: {e}"))?;

    let mut reader = BufReader::new(&mut conn);
    let mut line = String::new();
    reader
        .read_line(&mut line)
        .map_err(|e| format!("read status: {e}"))?;
    if !line.starts_with("OK") {
        return Err(format!("daemon: {}", line.trim()));
    }
    let mut body = String::new();
    reader
        .read_to_string(&mut body)
        .map_err(|e| format!("read body: {e}"))?;
    Ok(body)
}

fn fetch_status() -> Result<StatusResponse, String> {
    let body = request_key()?;
    serde_json::from_str(&body).map_err(|e| format!("parse status: {e}"))
}

fn render_day_time(status: &StatusResponse) -> String {
    let mut day_time = String::new();
    let symbols = get_symbols();
    match status.computed.sun.state.as_str() {
        "dawn" => day_time.push_str(symbols.dawn),
        "rising" => day_time.push_str(symbols.rising),
        "noon" => day_time.push_str(symbols.noon),
        "setting" => day_time.push_str(symbols.setting),
        "dusk" => day_time.push_str(symbols.dusk),
        _ => {}
    }
    day_time + &format!("{:.2}", status.computed.sun.next_event_in_hours)
}

fn bool_word(b: bool) -> &'static str {
    if b { "on" } else { "off" }
}

fn form_text(s: &StatusResponse) -> String {
    let mut parts: Vec<String> = Vec::new();
    let symbols = get_symbols();
    if s.computed.flags.lucia_phone_battery_low {
        parts.push(symbols.cell);
    }
    if s.computed.base_stations_on {
        parts.push(symbols.base_station);
    }
    if s.computed.kettle_on {
        parts.push(symbols.kettle);
    }
    if s.computed.bedroom_window_open {
        parts.push(symbols.window);
    }
    if s.computed.offloaded_compute_on {
        parts.push(symbols.offload_compute);
    }
    parts.push(render_day_time(s));
    parts.push(symbols.temp + &s.computed.den_temperature.to_string());
    return parts.join(" ");
}

fn form_tooltip(s: &StatusResponse) -> String {
    let mut tooltip = format!(
        "den: {} {}",
        s.computed.den_temperature.to_string(),
        s.computed.den_temperature_unit.trim()
    );
    tooltip += &format!("\nkettle: {}", bool_word(s.computed.kettle_on));
    tooltip += &format!(
        "\nbase stations: {}",
        bool_word(s.computed.base_stations_on)
    );
    tooltip += &format!(
        "\noffloaded compute: {}",
        bool_word(s.computed.offloaded_compute_on)
    );
    tooltip += &format!(
        "\nphone battery low: {}",
        bool_word(s.computed.flags.lucia_phone_battery_low)
    );
    tooltip += &format!(
        "\nbedroom window open: {}",
        bool_word(s.computed.bedroom_window_open)
    );
    tooltip += &format!("\nphase of day: {}", s.computed.sun.state);
    return tooltip;
}

fn form_css(s: &StatusResponse) -> String {
    if s.computed.flags.lucia_phone_battery_low || s.computed.flags.base_station_service_offline {
        return String::from("notify");
    } else {
        return String::from("ok");
    };
}

fn render(s: &StatusResponse) -> Output {
    Output {
        text: form_text(s),
        tooltip: form_tooltip(s),
        class: form_css(s),
    }
}

fn handle_socket_error(e: String) {
    let out = Output {
        text: "FATAL ERROR PARSING RESPONSE FROM UNIX SOCKET".to_string(),
        class: "unhealthy".to_string(),
        tooltip: e,
    };
    println!("{}", serde_json::to_string(&out).unwrap());
    std::process::exit(1);
}

fn main() {
    match fetch_status() {
        Ok(s) => println!("{}", serde_json::to_string(&render(&s)).unwrap()),
        Err(e) => handle_socket_error(e),
    }
}
