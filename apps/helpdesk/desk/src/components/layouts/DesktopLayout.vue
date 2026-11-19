<template>
  <div class="flex h-screen w-screen">
    <Sidebar />
    <div class="flex-1 flex flex-col h-full overflow-auto relative">
      <WorkspaceTabs v-if="!isCustomerPortal" />
      <AppHeader />
      <slot />
    </div>
    <Notifications />
  </div>
</template>
<script setup>
import { Notifications } from "@/components";
import AppHeader from "./AppHeader.vue";
import Sidebar from "./Sidebar.vue";
import WorkspaceTabs from "./WorkspaceTabs.vue";
import { isCustomerPortal } from "@/utils";
</script>
