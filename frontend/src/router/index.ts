import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '@/views/HomeView.vue'
import WorkflowView from '@/views/WorkflowView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView
    },
    {
      path: '/workflow/:id',
      name: 'workflow',
      component: WorkflowView,
      props: (route) => ({ id: Number(route.params.id) })
    },
    {
      path: '/office',
      name: 'office',
      component: () => import('@/views/VirtualOffice.vue')
    }
  ]
})

export default router
