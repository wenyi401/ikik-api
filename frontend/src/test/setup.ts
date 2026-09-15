// jsdom exposes Node's BroadcastChannel, whose Event implementation is not
// compatible with jsdom's MessageEvent. Browser synchronization is covered by
// the auth-session tests with an explicit transport stub.
Object.defineProperty(window, 'BroadcastChannel', {
  value: undefined,
  configurable: true
})

// jsdom 未实现 matchMedia，而 DataTable / 图表等组件在 setup 阶段就会调用它；
// 统一提供一个最小可用实现（matches=false，可注册/移除监听），
// 需要特定断言的用例可在自身 beforeEach 中覆盖。
if (typeof window !== 'undefined' && typeof window.matchMedia !== 'function') {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    configurable: true,
    value: (query: string): MediaQueryList =>
      ({
        matches: false,
        media: query,
        onchange: null,
        addListener: () => {},
        removeListener: () => {},
        addEventListener: () => {},
        removeEventListener: () => {},
        dispatchEvent: () => false,
      }) as unknown as MediaQueryList,
  })
}
