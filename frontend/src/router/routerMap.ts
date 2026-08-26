import ChatView from '@/views/chat/index.vue'
import ImageViewerView from '@/views/image-viewer/index.vue'
import SharedDriveView from '@/views/shared-drive/index.vue'

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
  {
    path: '/shared-drive',
    name: 'shared-drive',
    component: SharedDriveView,
  },
]

export default constantRouterMap
