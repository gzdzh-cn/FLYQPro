/// <reference types="vite/client" />

declare module '*.vue' {
  import { DefineComponent } from 'vue';
  // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/ban-types
  const component: DefineComponent<{}, {}, any>;
  export default component;
}
interface ImportMetaEnv {
  readonly VITE_API_BASE_URL: string;
}
declare interface Window {
  globalConfig: any
}
declare module '@arco-design/color'
declare module '@umoteam/editor'

declare module 'md5' {
  const content: any
  export = content
}
declare module 'qs' {
  const content: any
  export = content
}