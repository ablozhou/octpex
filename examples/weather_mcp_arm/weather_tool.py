"""
Weather Tool Definition & Live API Implementation
Demonstrates how standard Function Calling / MCP JSON Schemas map directly
to Octpex Tool abstractions and Octo-Arm capabilities.
Supports full names and abbreviations:
- 'c' or 'celsius': Celsius (e.g. Beijing)
- 'f' or 'fahrenheit': Fahrenheit
- 'd' or 'dual': Dual display (°F + °C, e.g. Los Angeles)
"""

import os
import json
import urllib.parse
import urllib.request
import urllib.error
from typing import Dict, Any, Tuple

from geo_skill import normalize_location_entity

# Supported unit abbreviations and canonical mapping
UNIT_NORMALIZATION = {
    "c": "celsius",
    "celsius": "celsius",
    "f": "fahrenheit",
    "fahrenheit": "fahrenheit",
    "d": "dual",
    "dual": "dual",
}

# Standard OpenAI / Ollama / MCP Tool Definition
WEATHER_TOOL_DEFINITION = {
    "type": "function",
    "function": {
        "name": "get_current_temperature",
        "description": (
            "Gets live current temperature and meteorological conditions for any global city or location in the world. "
            "The calling model (Brain/Ganglion) is expected to resolve natural language into standard geographical query keys "
            "(e.g. 'Washington' -> 'Washington, D.C.', 'Paris' -> 'Paris, France', 'Chaoyang' -> 'Chaoyang, Beijing', "
            "'New York' -> 'New York City', 'Washington State' -> 'State of Washington')."
        ),
        "parameters": {
            "type": "object",
            "properties": {
                "location": {
                    "type": "string",
                    "description": "Standardized city, municipality, or location name resolved by the model (e.g. 'Washington, D.C.', 'Beijing', 'Paris, France').",
                },
                "unit": {
                    "type": "string",
                    "enum": ["c", "f", "d", "celsius", "fahrenheit", "dual"],
                    "description": "Temperature unit: 'c' (celsius), 'f' (fahrenheit), or 'd' (dual °F + °C)",
                },
            },
            "required": ["location"],
        },
    },
}

# Backward compatibility alias for code reading top-level 'parameters' or 'name'
WEATHER_TOOL_SCHEMA = WEATHER_TOOL_DEFINITION["function"]

# Offline fallback dataset (used only if host has no internet connectivity)
OFFLINE_BACKUP_CACHE: Dict[str, Dict[str, Any]] = {
    "beijing": {"temp_c": 7.0, "temp_f": 44.6, "humidity": 30, "condition": "Partly Cloudy", "wind_kmh": 12.0},
    "los angeles": {"temp_c": 15.0, "temp_f": 59.0, "humidity": 65, "condition": "Patchy rain", "wind_kmh": 8.0},
    "san francisco": {"temp_c": 13.0, "temp_f": 55.4, "humidity": 75, "condition": "Foggy", "wind_kmh": 15.0},
    "london": {"temp_c": 5.0, "temp_f": 41.0, "humidity": 95, "condition": "Mist", "wind_kmh": 6.0},
    "tokyo": {"temp_c": 12.0, "temp_f": 53.6, "humidity": 55, "condition": "Clear", "wind_kmh": 9.0},
}


def validate_weather_parameters(params: Dict[str, Any]) -> Tuple[bool, str]:
    """
    Validates parameters against the JSON Schema, accepting both full names and abbreviations.
    Gracefully aliases 'city' to 'location' if provided by an LLM.
    """
    if not isinstance(params, dict):
        return False, "Parameters must be a dictionary"

    # Alias 'city' to 'location' if LLM generated 'city'
    if "location" not in params and "city" in params:
        params["location"] = params["city"]

    if "location" not in params:
        return False, "Missing required parameter: 'location'"

    location = params.get("location")
    if not isinstance(location, str) or not location.strip():
        return False, "Parameter 'location' must be a non-empty string"

    raw_unit = str(params.get("unit", "c")).strip().lower()
    if raw_unit not in UNIT_NORMALIZATION:
        # Default to 'c' if unknown
        params["unit"] = "c"

    return True, ""


def _fetch_from_wttr_live(location: str) -> Dict[str, Any]:
    """
    Queries live real-time observation from wttr.in JSON endpoint.
    """
    query_loc = location.strip()
    encoded_city = urllib.parse.quote(query_loc)
    url = f"https://wttr.in/{encoded_city}?format=j1"

    req = urllib.request.Request(
        url,
        headers={"User-Agent": "curl/8.0 (Octpex-WeatherArm)"},
    )

    with urllib.request.urlopen(req, timeout=6.0) as resp:
        if resp.status != 200:
            raise RuntimeError(f"Live weather API returned HTTP {resp.status}")
        raw_text = resp.read().decode("utf-8")
        data = json.loads(raw_text)

        current = data["current_condition"][0]
        temp_c = float(current["temp_C"])
        temp_f = float(current["temp_F"])
        humidity = int(current["humidity"])
        condition = current["weatherDesc"][0]["value"].strip()
        wind_kmh = float(current["windspeedKmph"])

        return {
            "temp_c": temp_c,
            "temp_f": temp_f,
            "humidity": humidity,
            "condition": condition,
            "wind_kmh": wind_kmh,
            "source": "wttr.in live meteorological API",
        }


def fetch_weather(location: str, unit: str = "c") -> Dict[str, Any]:
    """
    Executes live weather query via public meteorological APIs.
    Normalizes location through Geographic Entity Disambiguation Skill (geo_skill).
    Normalizes unit from shorthands ('c', 'f', 'd') or full names.
    Follows zero-trust principles: never silently returns fake dummy data when offline.
    """
    loc_clean = location.strip()
    raw_unit = str(unit).strip().lower()
    canonical_unit = UNIT_NORMALIZATION.get(raw_unit, "celsius")

    # 1. Apply Geographic Entity Disambiguation & Normalization Skill
    geo_entity = normalize_location_entity(loc_clean)
    query_target = geo_entity.query_target

    # 2. Attempt live meteorological API query
    live_data = None
    try:
        live_data = _fetch_from_wttr_live(query_target)
    except Exception as e:
        # Check if offline simulation is explicitly permitted by environment
        allow_offline_sim = os.environ.get("OCTPEX_ALLOW_OFFLINE_SIMULATION", "0").lower() in ("1", "true", "yes")
        if not allow_offline_sim:
            # Zero-trust principle: fail explicitly, NEVER silently return fake data
            raise ConnectionError(
                f"External weather API unreachable (wttr.in: {e}). "
                f"Octpex adheres to zero-trust principles and rejects synthetic dummy fallbacks. "
                f"Check network connectivity or set OCTPEX_ALLOW_OFFLINE_SIMULATION=1 for explicit simulation mode."
            )

        # Explicit offline simulation mode: clearly labeled so users and Brain know it's not live
        key = query_target.lower()
        backup = OFFLINE_BACKUP_CACHE.get(
            key,
            {"temp_c": 10.0, "temp_f": 50.0, "humidity": 50, "condition": "Clear (Simulated)", "wind_kmh": 10.0},
        )
        live_data = dict(backup)
        live_data["source"] = f"OFFLINE_SIMULATION [Offline Simulated - No Internet] ({type(e).__name__}: {e})"
        live_data["is_simulated"] = True

    temp_c = round(float(live_data["temp_c"]), 1)
    temp_f = round(float(live_data["temp_f"]), 1)

    is_sim = live_data.get("is_simulated", False)
    sim_tag = " [Offline Simulated - No Internet]" if is_sim else ""

    if canonical_unit == "dual":
        primary_temp = temp_f
        display_str = f"{temp_f}°F ({temp_c}°C){sim_tag}"
    elif canonical_unit == "fahrenheit":
        primary_temp = temp_f
        display_str = f"{temp_f}°F{sim_tag}"
    else:  # celsius
        primary_temp = temp_c
        display_str = f"{temp_c}°C{sim_tag}"

    return {
        "location": loc_clean,
        "query_target": query_target,
        "canonical_name": geo_entity.canonical_name,
        "english_name": geo_entity.english_name,
        "province_or_state": geo_entity.province_or_state,
        "country": geo_entity.country,
        "disambiguation_note": geo_entity.disambiguation_note,
        "temperature": primary_temp,
        "unit": raw_unit,
        "canonical_unit": canonical_unit,
        "temp_c": temp_c,
        "temp_f": temp_f,
        "display": display_str,
        "humidity": live_data["humidity"],
        "condition": live_data["condition"],
        "wind_kmh": live_data["wind_kmh"],
        "data_source": live_data["source"],
        "is_simulated": is_sim,
    }
