<template>
  <div class="titlebar">
    <span class="app-name">reqost</span>
  </div>
</template>

<style scoped>
.titlebar {
  height: 50px;
  flex-shrink: 0;
  background: var(--rail-bg);
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  /* macOS traffic lights live in the leftmost ~80px */
  padding-left: 88px;
  -webkit-app-region: drag;
  user-select: none;
  -webkit-user-select: none;
}

.app-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-faint);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  -webkit-app-region: no-drag;
}
</style>
