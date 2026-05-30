# Mobile Shared

移动端共享代码目录。

## Contents

- `api`: HTTP client and token persistence.
- `types`: API DTOs shared across screens.
- `ui`: shared presentational components.
- `utils`: formatting helpers.

业务状态和页面交互应放在 `src/features/*`，不要继续堆回 `App.tsx`。
