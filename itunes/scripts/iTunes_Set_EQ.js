
function run(argv) {
    const Music = Application("com.apple.Music");

    // Helper function to parse command-line arguments
    function parseArgs(argv) {
        const args = {};
        for (let i = 0; i < argv.length; i++) {
            if (argv[i].startsWith('--')) {
                const key = argv[i].substring(2);
                const value = (i + 1 < argv.length && !argv[i + 1].startsWith('--')) ? argv[i + 1] : true;
                args[key] = value;
            }
        }
        return args;
    }

    const args = parseArgs(argv);

    // Logic to set EQ
    if (args.hasOwnProperty('enabled')) {
        const requestedState = (args.enabled === 'true' || args.enabled === true);
        Music.eqEnabled.set(requestedState);
    }

    if (args.hasOwnProperty('preset')) {
        const presetName = args.preset;
        try {
            const presetToSet = Music.EQPresets.byName(presetName);
            Music.currentEQPreset.set(presetToSet);
            Music.eqEnabled.set(true); // Applying a preset always enables EQ
        } catch (e) {
            return JSON.stringify({
                error: "Preset not found",
                preset_name: presetName
            });
        }
    }

    // Return the new state of the EQ
    const eq = {
        enabled: Music.eqEnabled(),
        current_preset: null,
        available_presets: []
    };

    if (eq.enabled) {
        try {
            eq.current_preset = Music.currentEQPreset().name();
        } catch (e) {
            eq.current_preset = null;
        }
    }

    const presets = Music.EQPresets;
    for (let i = 0; i < presets.length; i++) {
        eq.available_presets.push(presets[i].name());
    }

    return JSON.stringify(eq);
}
