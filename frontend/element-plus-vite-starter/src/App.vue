<template>
  <el-config-provider namespace="ep">
<!--      导航栏 顶部-->
    <BaseHeader />
    <el-main>
<!--          用来承接路由表路由结果返回的组件-->
        <el-backtop :right="100" :bottom="100" />
        <router-view></router-view>
    </el-main>
  </el-config-provider>
</template>

<style>
#app {
  text-align: center;
  color: var(--ep-text-color-primary);
}

.main-container {
  height: calc(100vh - var(--ep-menu-item-height) - 3px);
}
</style>
