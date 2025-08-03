# EQ and Audio Output Tools - Issues and Fixes

## Overview

This document summarizes the issues found with the iTunes MCP tools for EQ control and audio output device management, along with the fixes applied.

## Issues Identified

### 1. EQ Scripts Problems

#### iTunes_Get_EQ.js
- **Problem**: The script was returning an empty `available_presets` array despite having 23 EQ presets available in Apple Music.
- **Root Cause**: Incorrect JXA syntax for accessing Music app collections. The script was using `Music.EQPresets()` and `presets.at(i)` which are not valid JXA constructs.
- **Fix**: Changed to use `Music.eqPresets` (camelCase, no parentheses) and array indexing with `presets[i]`.

#### iTunes_Set_EQ.js
- **Problem**: Setting EQ presets was failing with "Preset not found" error even for valid preset names like "Rock".
- **Root Cause**: Same incorrect collection access pattern, plus using `Music.EQPresets.byName()` which doesn't exist in JXA.
- **Fix**: Implemented proper iteration through the presets collection to find matching preset by name.

### 2. Audio Output Scripts Problems

#### All Audio Output Scripts
- **Problem**: Scripts were unreliable and throwing privilege violations when trying to access AirPlay devices.
- **Root Causes**:
  1. Mixing JavaScript and AppleScript via shell commands (`app.doShellScript('osascript -e "..."')`)
  2. Incorrect property access for AirPlay devices collection
  3. System privilege restrictions on enumerating AirPlay devices

#### iTunes_Get_Audio_Output.js
- **Fix**: Simplified to check `Music.airPlayEnabled()` and `Music.currentAirPlayDevices()` instead of trying to enumerate all devices.

#### iTunes_List_AirPlay_Devices.js
- **Fix**: Modified to only show local device and indicate if AirPlay is active, without trying to enumerate specific AirPlay devices due to system restrictions.

#### iTunes_Set_AirPlay_Device.js
- **Fix**: Limited functionality to only switching between local output (by setting `airPlayEnabled = false`) and informing users about the limitation.

### 3. General Architecture Issues

- **Inconsistent Application References**: Some scripts used `Application("com.apple.Music")` while others used `Application('Music')`.
- **Poor Error Handling**: Scripts would fail completely instead of gracefully degrading.
- **Unnecessary Dependencies**: Scripts were importing `ObjC.import('stdlib')` without using it.

## Key Fixes Applied

### 1. Correct JXA Syntax for Collections

```javascript
// WRONG - doesn't work
const presets = Music.EQPresets()
const preset = presets.at(i)

// CORRECT
const presets = Music.eqPresets  // Note: camelCase, no parentheses
const preset = presets[i]        // Standard array indexing
```

### 2. Proper Error Handling

All scripts now include try-catch blocks with fallback behavior:
- EQ scripts return current state even if some operations fail
- Audio output scripts gracefully degrade to showing only local output if AirPlay access fails

### 3. Simplified AirPlay Handling

Due to macOS privilege restrictions on accessing AirPlay devices programmatically:
- Removed attempts to enumerate all AirPlay devices
- Focused on detecting whether AirPlay is enabled/active
- Can only toggle between local and AirPlay (not select specific AirPlay devices)

### 4. Consistent Code Style

- Unified to use `Application('Music')` throughout
- Removed unnecessary imports and dependencies
- Standardized JSON response format with proper error messages

## Current Limitations

1. **AirPlay Device Enumeration**: Cannot list all available AirPlay devices due to macOS security restrictions.
2. **AirPlay Device Selection**: Cannot programmatically select a specific AirPlay device by name.
3. **AirPlay Functionality**: Limited to:
   - Detecting if AirPlay is active
   - Switching to local output by disabling AirPlay
   - Getting the name of currently active AirPlay device (sometimes)

## Testing Results

After fixes:
- ✅ EQ status retrieval works correctly, showing all 23 presets
- ✅ EQ preset setting works for all valid preset names
- ✅ Audio output detection works (local vs AirPlay)
- ✅ Scripts handle errors gracefully without crashing
- ⚠️ AirPlay device enumeration limited due to system restrictions

## Recommendations

1. **Documentation Update**: Update the MCP tool descriptions to clearly indicate the AirPlay limitations.
2. **User Guidance**: Inform users that they need to manually select AirPlay devices in the Music app before using these tools.
3. **Alternative Approach**: Consider using System Events UI scripting for more complete AirPlay control, though this would require accessibility permissions.
