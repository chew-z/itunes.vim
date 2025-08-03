
function run() {
    const Music = Application("com.apple.Music");

    if (!Music.running()) {
        return JSON.stringify({ error: "Music app is not running." });
    }

    try {
        const devices = Music.airPlayDevices();
        const deviceList = [];

        for (let i = 0; i < devices.length; i++) {
            const device = devices[i];
            deviceList.push({
                name: device.name(),
                kind: device.kind(),
                selected: device.selected(),
                sound_volume: device.soundVolume()
            });
        }

        return JSON.stringify(deviceList);

    } catch (e) {
        return JSON.stringify({ error: `An error occurred: ${e.message}` });
    }
}
