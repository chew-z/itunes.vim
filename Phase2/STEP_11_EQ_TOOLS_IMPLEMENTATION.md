# Implementation Plan: Add EQ Control Tools (`check_eq`, `set_eq`)

**Objective:** Implement two new MCP tools, `check_eq` and `set_eq`, to allow an LLM to check and control the Apple Music Equalizer settings. The implementation must handle enabling/disabling the EQ and setting specific, predefined presets without allowing for manual band adjustments.

---

### Phase 1: JXA Script Development

Create two new JavaScript for Automation (JXA) scripts in the `itunes/scripts/` directory. These scripts must communicate with the Go application by printing a single JSON object to standard output.

#### 1. Create `itunes/scripts/iTunes_Get_EQ.js`

This script fetches the current EQ status.

**Logic:**
1.  Access the Music application object: `const Music = Application("com.apple.Music");`
2.  Define a data structure to hold the EQ status.
3.  Check if the EQ is enabled using `Music.eqEnabled()`.
4.  If enabled, get the current preset name via `Music.currentEQPreset().name()`. If disabled, the current preset should be `null`.
5.  Get a list of all available preset names by mapping over `Music.EQPresets()`.
6.  Print a single JSON object containing the `enabled` status, `current_preset`, and `available_presets` to `console.log`.

**Example JSON Output:**
```json
{
  "enabled": true,
  "current_preset": "Jazz",
  "available_presets": ["Acoustic", "Bass Booster", "Classical", "Jazz", "Latin", "Loudness", "Rock", "Small Speakers", "Spoken Word", "Vocal Booster"]
}
```

#### 2. Create `itunes/scripts/iTunes_Set_EQ.js`

This script modifies the EQ state based on command-line arguments and returns the new state.

**Argument Handling:**
- The script must be able to parse arguments passed from the `osascript` command. A simple argument parsing function will be needed to handle `--preset "Preset Name"` and `--enabled true/false`.

**Logic:**
1.  **To Disable EQ**: If the `--enabled false` argument is passed, the script must execute `Music.eqEnabled = false`. This action should take precedence.
2.  **To Enable EQ**: If `--enabled true` is passed without a preset, it should execute `Music.eqEnabled = true`.
3.  **To Set a Preset**:
    *   If a `--preset "PRESET_NAME"` argument is passed, find the corresponding preset object in `Music.EQPresets()`.
    *   If the preset is found, set `Music.currentEQPreset` to the found preset object.
    *   Crucially, after setting the preset, ensure the EQ is enabled by setting `Music.eqEnabled = true`.
4.  After performing the action, the script must fetch the new EQ status and print it to `console.log` in the exact same JSON format as `iTunes_Get_EQ.js` for confirmation.

---

### Phase 2: Go Application Integration

Integrate the new JXA scripts into the Go application.

#### 1. Embed Scripts (`itunes/scripts.go`)
- Add the two new filenames (`iTunes_Get_EQ.js`, `iTunes_Set_EQ.js`) to the `go:embed` directive to bundle them into the application binary.

#### 2. Update Core Library (`itunes/itunes.go`)
- **Define `EQStatus` Struct**: Create a new Go struct to unmarshal the JSON output from the JXA scripts. Use a pointer for `CurrentPreset` to handle the `null` case.
  ```go
  type EQStatus struct {
      Enabled          bool     `json:"enabled"`
      CurrentPreset    *string  `json:"current_preset"`
      AvailablePresets []string `json:"available_presets"`
  }
  ```
- **Implement `GetEQStatus()` function**:
  - This function will take no arguments.
  - It will execute the `iTunes_Get_EQ.js` script using the existing `runScript` helper.
  - It will unmarshal the JSON output into an `*EQStatus` struct and return it.
- **Implement `SetEQStatus(preset string, enabled *bool)` function**:
  - This function will accept an optional `preset` name (string) and an optional `enabled` state (pointer `*bool` to distinguish between `false` and not provided).
  - It will dynamically build the `osascript` command arguments based on the provided parameters (e.g., `"-e", script, "--preset", preset`).
  - It will execute the `iTunes_Set_EQ.js` script with the constructed arguments.
  - It will unmarshal the JSON response from the script into an `*EQStatus` struct and return it as confirmation.

---

### Phase 3: MCP Server Exposure

Expose the new functionality as tools in the MCP server.

#### 1. Update MCP Server (`mcp-server/main.go`)
- **Register `check_eq` Tool**:
  - **Description**: "Check the current Apple Music Equalizer (EQ) status, including the active preset and a list of all available presets."
  - **Parameters**: None.
  - **Handler**: The handler function will call `itunes.GetEQStatus()` and return the result.
- **Register `set_eq` Tool**:
  - **Description**: "Set the Apple Music Equalizer (EQ). Can be used to enable/disable the EQ or apply a specific preset."
  - **Parameters**:
    - `preset` (string, optional): The name of the EQ preset to apply (e.g., "Rock", "Jazz"). Applying a preset will automatically enable the EQ.
    - `enabled` (boolean, optional): Set to `true` to enable the EQ or `false` to disable it.
  - **Handler**: The handler will extract the `preset` and `enabled` parameters and call `itunes.SetEQStatus()` with them.

---

### Phase 4: Documentation

Update the project documentation to reflect the new tools.

#### 1. Update `CLAUDE.md`
- Add sections for the `check_eq` and `set_eq` tools under the "MCP Tools" heading.
- For each tool, provide its description, parameters, return value structure, and clear usage examples for all scenarios (checking, setting a preset, enabling, disabling).

---

### Final Verification

1.  Run `go build -o bin/mcp-itunes ./mcp-server` to compile the server.
2.  Run the server and test the new tools to ensure they behave as expected.
