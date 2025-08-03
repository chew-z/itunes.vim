function run(argv) {
    const Music = Application('Music')

    // Helper function to parse command-line arguments
    function parseArgs(argv) {
        const args = {}
        for (let i = 0; i < argv.length; i++) {
            if (argv[i].startsWith('--')) {
                const key = argv[i].substring(2)
                const value = i + 1 < argv.length && !argv[i + 1].startsWith('--') ? argv[i + 1] : true
                args[key] = value
                if (value !== true) i++ // Skip the value in the next iteration
            }
        }
        return args
    }

    try {
        const args = parseArgs(argv)

        // Handle enabling/disabling EQ
        if (args.hasOwnProperty('enabled')) {
            const requestedState = args.enabled === 'true' || args.enabled === true
            Music.eqEnabled = requestedState
        }

        // Handle preset setting
        if (args.hasOwnProperty('preset') && args.preset !== true) {
            const presetName = args.preset
            let presetFound = false

            try {
                // Get all EQ presets
                const presets = Music.eqPresets

                // Search for the preset by name
                for (let i = 0; i < presets.length; i++) {
                    const preset = presets[i]
                    if (preset.name() === presetName) {
                        Music.currentEQPreset = preset
                        Music.eqEnabled = true // Applying a preset always enables EQ
                        presetFound = true
                        break
                    }
                }

                if (!presetFound) {
                    // Build list of available presets for error message
                    const availablePresets = []
                    for (let i = 0; i < presets.length; i++) {
                        try {
                            availablePresets.push(presets[i].name())
                        } catch (e) {
                            continue
                        }
                    }

                    return JSON.stringify({
                        error: 'Preset not found',
                        preset_name: presetName,
                        available_presets: availablePresets,
                    })
                }
            } catch (e) {
                // If we can't access presets normally, return error
                return JSON.stringify({
                    error: 'Failed to access EQ presets',
                    preset_name: presetName,
                    message: e.toString(),
                })
            }
        }

        // Return the new state of the EQ
        const result = {
            enabled: false,
            current_preset: null,
            available_presets: [],
        }

        // Get current EQ state
        result.enabled = Music.eqEnabled()

        // Get current preset if enabled
        if (result.enabled) {
            try {
                const currentPreset = Music.currentEQPreset()
                if (currentPreset) {
                    result.current_preset = currentPreset.name()
                }
            } catch (e) {
                result.current_preset = null
            }
        }

        // Get all available presets
        try {
            const presets = Music.eqPresets
            if (presets && presets.length > 0) {
                for (let i = 0; i < presets.length; i++) {
                    try {
                        const preset = presets[i]
                        result.available_presets.push(preset.name())
                    } catch (e) {
                        continue
                    }
                }
            }
        } catch (e) {
            // If direct access fails, try alternative method
            try {
                const presetNames = Music.eqPresets.name()
                if (Array.isArray(presetNames)) {
                    result.available_presets = presetNames
                } else if (presetNames) {
                    result.available_presets = [presetNames]
                }
            } catch (e2) {
                // Return empty array if all methods fail
                result.available_presets = []
            }
        }

        return JSON.stringify(result)
    } catch (e) {
        return JSON.stringify({
            error: 'Failed to set EQ status',
            message: e.toString(),
        })
    }
}
