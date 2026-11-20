<!-- KD: shared page shell - title bar with actions, then a scrolling body. -->
<template>
  <div class="flex h-full min-h-0 flex-col overflow-hidden rounded-xl border border-border bg-background">
    <header class="flex min-h-14 shrink-0 flex-wrap items-center gap-3 border-b border-border px-5 py-2.5">
      <slot name="leading" />
      <div class="min-w-0 flex-1">
        <h1 class="truncate text-base font-semibold leading-tight">{{ title }}</h1>
        <p v-if="subtitle" class="truncate text-xs text-muted-foreground">{{ subtitle }}</p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <slot name="actions" />
      </div>
    </header>
    <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain" :class="bodyClass">
      <slot />
    </div>
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, required: true },
  subtitle: { type: String, default: '' },
  bodyClass: { type: [String, Array, Object], default: '' }
})
</script>
