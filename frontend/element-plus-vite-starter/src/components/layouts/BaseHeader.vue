<script lang="ts" setup>
import { toggleDark } from "~/composables";
</script>

<template>
  <el-menu
          class="el-menu-docker-db"
          :default-active="this.$route.path"
          router
          mode="horizontal"
          :ellipsis="false"
  >
    <el-menu-item index="/home">Docker镜像信息库</el-menu-item>
    <el-menu-item index="/repositories">Repositories</el-menu-item>
    <el-menu-item index="/images">Images</el-menu-item>

<!--    以下内容置右-->
    <div class="flex-grow" />
    <el-menu-item h="full" @click="toggleDark()">
      <button
        class="border-none w-full bg-transparent cursor-pointer"
        style="height: var(--ep-menu-item-height)"
      >
        <i inline-flex i="dark:ep-moon ep-sunny" />
      </button>
    </el-menu-item>
  </el-menu>
</template>

<style scoped>
.flex-grow {
    flex-grow: 1;
}
</style>
