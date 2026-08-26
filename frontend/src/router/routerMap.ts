import ChatView from '@/views/chat/index.vue'
import ImageViewerView from '@/views/image-viewer/index.vue'

const constantRouterMap = [
  {
    path: '/',
    name: 'chat',
    component: ChatView,
  },
  {
    path: '/image-viewer',
    name: 'image-viewer',
    component: ImageViewerView,
  },
]

export default constantRouterMap
