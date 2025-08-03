function run() {
    const Music = Application('Music')

    try {
        const result = {
            enabled: false,
            current_preset: null,
            available_presets: [],
        }

        // Get EQ enabled status
        result.enabled = Music.eqEnabled()

        // Get current EQ preset if enabled
        if (result.enabled) {
            try {
                const currentPreset = Music.currentEQPreset()
                if (currentPreset) {
                    result.current_preset = currentPreset.name()
                }
            } catch (e) {
                // EQ might be enabled but no preset selected
                result.current_preset = null
            }
        }

        // Get all available EQ presets
        // In JXA, we need to use the eqPresets property (no space)
        try {
            const presets = Music.eqPresets
            if (presets && presets.length > 0) {
                for (let i = 0; i < presets.length; i++) {
                    try {
                        const preset = presets[i]
                        result.available_presets.push(preset.name())
                    } catch (e) {
                        // Skip if we can't access this preset
                        continue
                    }
                }
            }
        } catch (e) {
            // If direct access fails, try using whose clause
            try {
                const presetNames = Music.eqPresets.name()
                if (Array.isArray(presetNames)) {
                    result.available_presets = presetNames
                } else if (presetNames) {
                    // Single preset case
                    result.available_presets = [presetNames]
                }
            } catch (e2) {
                // Last resort: return empty array
                result.available_presets = []
            }
        }

        return JSON.stringify(result)
    } catch (e) {
        return JSON.stringify({
            error: 'Failed to get EQ status',
            message: e.toString(),
            enabled: false,
            current_preset: null,
            available_presets: [],
        })
    }
}
