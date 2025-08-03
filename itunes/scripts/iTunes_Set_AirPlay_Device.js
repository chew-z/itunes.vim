
function run(argv) {
    const Music = Application("com.apple.Music");

    if (argv.length !== 1) {
        return JSON.stringify({ error: "A single device name argument is required." });
    }
    const targetDeviceName = argv[0];

    if (!Music.running()) {
        return JSON.stringify({ error: "Music app is not running." });
    }

    try {
        const devices = Music.airPlayDevices();
        let targetDevice = null;

        // First, find the target device
        for (let i = 0; i < devices.length; i++) {
            if (devices[i].name() === targetDeviceName) {
                targetDevice = devices[i];
                break;
            }
        }

        if (!targetDevice) {
            return JSON.stringify({ error: `Device named '${targetDeviceName}' not found.` });
        }

        // Deselect all other devices and select the target
        for (let i = 0; i < devices.length; i++) {
            devices[i].selected.set(devices[i].name() === targetDeviceName);
        }

        // Return the new active device
        return JSON.stringify({
            name: targetDevice.name(),
            kind: targetDevice.kind(),
            selected: targetDevice.selected(),
            sound_volume: targetDevice.soundVolume()
        });

    } catch (e) {
        return JSON.stringify({ error: `An error occurred: ${e.message}` });
    }
}
