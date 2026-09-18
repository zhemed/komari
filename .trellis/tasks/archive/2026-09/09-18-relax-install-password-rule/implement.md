# 执行计划

1. 后端 `web/install/install.go`：删 `hasStrongPassword` 调用与函数（保留长度 8–256）。
2. 后端测试 `web/install/install_test.go`：
   - `TestInstallRejectsWeakPasswordWithoutCreatingAccount` → 改写为
     `TestInstallAcceptsPasswordWithoutUppercase`（`lowercaseonly1` 现在应 200 且用户存在）；
   - 长度下限仍由 `TestInstallRejectsInvalidInputWithoutCreatingAccount`（"short" → 400）覆盖。
3. 前端：删 `install.tsx` 与 `admin/account.tsx` 里的复杂度正则各一行。
4. 5 语言包删 `password_strength_error` 键（保格式，删后逐个 `json.load` 校验）。
5. `./scripts/build-frontend.sh`（会因哈希不符失败并打印新值）→ 回填
   `scripts/frontend-build.env` 的 `FRONTEND_TREE_SHA256` → 再跑一次确认通过（顺带证明可复现）。
6. `docs/MAINTAINING.md` §4 增一行记录该分歧。
7. 验证：`go test ./web/install/...`、`./scripts/check-repo.sh --full`、grep 死键归零。
8. 向用户申请发版（构建/推送/镜像 = 网络动作）；获批后版本 +1、构建 18 资产 + 两镜像、gh release。
9. journal + `task.py finish` + `archive`。
