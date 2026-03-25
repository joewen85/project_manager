# Frontend UI 规范（H12）

## 1. 设计基线
- 风格：SaaS 极简管理后台
- 技术：React + TypeScript + 全局 CSS 设计变量
- 主题变量入口：`frontend/src/styles.css`

## 2. 颜色与状态
- 主色：`--color-primary`
- 危险色：`--color-danger`
- 状态色来源：`frontend/src/constants/status.ts`
  - `pending / queued / processing / completed`
- 统计图、甘特图、任务树必须复用同一状态色

## 3. 间距与圆角
- 间距变量：`--space-1 ~ --space-5`
- 圆角变量：`--radius-md`、`--radius-lg`
- 页面主容器统一：`.page-section`

## 4. 组件规范
- 按钮：统一 `.btn`，状态按钮使用 `.btn.secondary/.btn.danger`
- 表单：必填标签使用 `.required-label`
- 列表状态：统一使用 `DataState` 组件
- 分页：统一使用 `Pagination` 组件
- 弹窗：统一使用 `Modal` 组件

## 5. 交互规范
- 所有可交互元素保留 hover/focus/active 反馈
- 键盘焦点统一 `focus-visible`
- 动效时长与缓动统一使用变量：
  - `--duration-fast`
  - `--duration-base`
  - `--ease-standard`
- 必须兼容 `prefers-reduced-motion`

## 6. 响应式规范
- 断点：`1440 / 1024 / 768 / 480`
- 规则：
  - `1024` 以下工具栏进入双列或单列
  - `768` 以下表格允许横向滚动
  - `480` 以下压缩卡片与顶栏尺寸

## 7. 新页面接入要求
新增页面应至少满足：
1) 使用 `page-section` 包裹  
2) 列表页接入 `DataState + Pagination`  
3) 表单页接入 `required-label + 提交态 + 错误反馈`  
4) 图表/状态展示复用 `STATUS_META`
