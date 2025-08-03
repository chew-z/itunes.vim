function run() {
    const Music = Application('Music')

    if (!Music.running()) {
        return JSON.stringify({ error: 'Music app is not running.' })
    }

    try {
        const deviceList = []

        // Get the computer name for local output
        const app = Application.currentApplication()
        app.includeStandardAdditions = true
        let computerName = 'Computer'

        try {
            computerName = app.doShellScript('scutil --get ComputerName')
        } catch (e) {
            try {
                computerName = app.doShellScript('hostname -s')
            } catch (e2) {
                // Use default
            }
        }

        // Get current sound volume
        const soundVolume = Music.soundVolume()

        // Always add the local device as an option
        deviceList.push({
            name: computerName,
            kind: 'computer',
            selected: true, // Will be updated if AirPlay is active
            sound_volume: soundVolume,
        })

        // Try to check AirPlay status
        try {
            const airPlayEnabled = Music.airPlayEnabled()

            // If AirPlay is enabled, we know at least one AirPlay device is selected
            if (airPlayEnabled) {
                // Mark local as not selected since AirPlay is active
                deviceList[0].selected = false

                // Note: We can't reliably enumerate all AirPlay devices due to privilege restrictions
                // But we can indicate that AirPlay is active
                deviceList.push({
                    name: 'AirPlay Device',
                    kind: 'AirPlay device',
                    selected: true,
                    sound_volume: soundVolume,
                    note: 'Unable to enumerate specific AirPlay devices due to system restrictions',
                })
            }
        } catch (e) {
            // AirPlay status couldn't be determined, assume local only
        }

        return JSON.stringify(deviceList)
    } catch (e) {
        // Return minimal info on any error
        return JSON.stringify([
            {
                name: 'Computer',
                kind: 'computer',
                selected: true,
                sound_volume: 100,
                error: 'Failed to enumerate audio devices',
            },
        ])
    }
}
