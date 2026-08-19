const constantRouterMap = [
  {
    path: '/',
    name: 'chat',
    component: () => import('@/views/chat/index.vue'),
  },
]

export default constantRouterMap
