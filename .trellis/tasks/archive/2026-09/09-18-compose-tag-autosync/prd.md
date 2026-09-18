# compose tag 自动同步（面板内建，方案 A）

## Goal

B 策略（挂 socket 重建容器）下，面板升级后 compose 文件里的 `image:` tag 会落后；
之后**任何一次文件编辑**都会按旧 tag 把版本拉回去（§15.4 实测）。用户要求全自动。

方案 A：升级成功后由 **helper** 顺手把 compose 文件里对应 service 的 `image:` 改成新版本。

## Requirements

- **R1 检测**：从自身容器 labels 读 `com.docker.compose.project.config_files` / `com.docker.compose.service`；
  拿不到（非 compose 部署）就完全跳过，不留副作用。
- **R2 挂载**：helper 需要读写该文件 → 把 compose 文件**所在目录**（不是文件本身，便于原子替换）
  以同路径 RW 挂进 helper。
- **R3 改写**：只改目标 service 块内的 `image:` 行；保留行尾注释与缩进；旧值含 `${…}`（变量）
  或找不到 service → 跳过并记日志；**改动前留 `.bak`**；写临时文件 + `os.Rename` 原子替换；
  值已相同 → 不动文件（幂等）。
- **R4 时机**：新容器**启动成功之后**才同步；同步失败**不影响**升级结果（只记日志）。
- **R5 测试**：改写函数的单元测试覆盖（基础替换/注释保留/多 service/幂等/变量跳过/找不到 service/
  备份生成），且**非永真**（改坏了要能红）；`helperBinds` 增加挂载的单元测试。
- **R6 E2E**：真实 compose 项目（钉 0.0.17 + socket）里通过**面板 RPC** 升级到 0.0.16，
  验证 compose 文件里的 tag 被改成 0.0.16，且容器与文件一致。
- **R7 文档**：MAINTAINING §15.4 更新（≥0.0.18 起自动同步，手动那一步不再需要）+ README 注释。
- **R8** 需要发一版才能到线上（用户已知），发版前先问。

## Acceptance Criteria

- [ ] 单元测试含上述 6 类用例，`go test ./internal/upgrade/...` 全绿
- [ ] E2E：面板升级后 compose 文件 tag 从 0.0.17 变 0.0.16，`.bak` 存在，容器版本与文件一致
- [ ] 非 compose 部署不受影响（helper 不挂额外目录、不写文件）
- [ ] `./scripts/check-repo.sh --full` 全绿；提交带 `[task:compose-tag-autosync]`
- [ ] journal + 归档

## Out of Scope

- 不改升级的其它行为（下载/校验/替换/回滚）；
- 不处理 `docker compose -f a.yml -f b.yml` 的多文件叠加（只认含目标 service 的第一个文件，
  认不出就跳过）；不处理变量形式的 image。
