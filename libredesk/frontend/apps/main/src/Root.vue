<template>
  <TooltipProvider :delay-duration="150">
    <Toaster
      class="pointer-events-auto"
      position="top-center"
      close-button
      :theme="mode === 'auto' ? 'system' : mode"
    />
    <RouterView />
  </TooltipProvider>
</template>

<script setup>
import { useColorMode } from '@vueuse/core'
import { RouterView } from 'vue-router'
import { Toaster } from '@shared-ui/components/ui/sonner'
import { TooltipProvider } from '@shared-ui/components/ui/tooltip'

// KD: light theme by default, like Zendesk. Applied once per browser, so an
// agent who later picks dark/system in their menu keeps that choice.
try {
  if (!localStorage.getItem('kd_theme_default_applied')) {
    localStorage.setItem('vueuse-color-scheme', 'light')
    localStorage.setItem('kd_theme_default_applied', '1')
  }
} catch {
  // storage blocked: useColorMode below still defaults to light
}
const mode = useColorMode({ initialValue: 'light' })
</script>
