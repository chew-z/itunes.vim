
function run() {
    const Music = Application("com.apple.Music");
    const eq = {
        enabled: Music.eqEnabled(),
        current_preset: null,
        available_presets: []
    };

    if (eq.enabled) {
        try {
            eq.current_preset = Music.currentEQPreset().name();
        } catch (e) {
            // In some edge cases (like just after startup),
            // eq can be enabled but no preset is active.
            eq.current_preset = null;
        }
    }

    const presets = Music.EQPresets;
    for (let i = 0; i < presets.length; i++) {
        eq.available_presets.push(presets[i].name());
    }

    return JSON.stringify(eq);
}
