import { createRouter, createWebHashHistory } from 'vue-router'
import SpeciesView from './views/SpeciesView.vue'
import PresetsView from './views/PresetsView.vue'
import SkinsView from './views/SkinsView.vue'
import SkinDemoView from './views/SkinDemoView.vue'

export default createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', redirect: '/species' },
    { path: '/species', name: 'species', component: SpeciesView },
    { path: '/presets', name: 'presets', component: PresetsView },
    { path: '/skins', name: 'skins', component: SkinsView },
    { path: '/skin', name: 'skin', component: SkinDemoView },
  ],
})
