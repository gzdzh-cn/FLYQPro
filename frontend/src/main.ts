import { createApp } from 'vue'
import App from './App.vue'
import Router from './router/index';
import store from './store';
import ArcoVue from '@arco-design/web-vue';
import ArcoVueIcon from '@arco-design/web-vue/es/icon';
import '@arco-design/web-vue/dist/arco.css';
import '@/assets/style/global.less';
const app = createApp(App);
app.use(Router);
app.use(store);
app.use(ArcoVue, {});
app.use(ArcoVueIcon);
app.mount('#app');
