import { createRouter, createWebHashHistory } from "vue-router";
import HomeView from "../views/HomeView.vue";
import RepositoriesView from "../views/RepositoriesView.vue";
import ImagesView from "../views/ImagesView.vue";


const routes = [
    {
        path: '/home',  // 程序启动默认路由
        component: HomeView,
    },
    {
        path: '/repositories',
        component: RepositoriesView,
    },
    {
        path: '/images',
        component: ImagesView,
    },
    {
        path: '/',
        redirect: '/repositories',
    },
];

const router = createRouter({
    history: createWebHashHistory(),
    routes: routes,
});

export default router;
