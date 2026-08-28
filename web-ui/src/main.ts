import { mount } from 'svelte'
import '@fontsource-variable/familjen-grotesk'
import '@fontsource/ibm-plex-mono/400.css'
import '@fontsource/ibm-plex-mono/500.css'
import '@fontsource/ibm-plex-mono/600.css'
import './app.css'
import { maybeInstallMockFromEnv } from '$lib/api/mock'
import App from './App.svelte'

maybeInstallMockFromEnv()

const app = mount(App, { target: document.getElementById('app')! })

export default app
