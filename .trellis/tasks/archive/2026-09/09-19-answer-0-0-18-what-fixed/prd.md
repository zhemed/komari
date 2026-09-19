# 回答：0.0.18 修复了什么（只读）

## Goal

用户提问 0.0.18 修复内容；只读查询：读保存的 release notes、0.0.18 标签的 git 历史与 diff、
GitHub 已发布 release 正文、当前仓库 0.0.17 代码行为，整理成答复。禁止改动任何文件/发布物/生产。

## Requirements

- 以事实为准：已发布 release 正文 + `0.0.18` 标签实际 diff + 当前仓库 0.0.17 的代码行为。
- 必须点明当前状态：弧线已全面回滚，仓库与生产都是 0.0.17，这些修复**现在不在运行**。

## Acceptance Criteria

- [x] 列出 0.0.18 的两条真 bug 修复与新增能力
- [x] 每条给出代码/发布正文依据（文件、行号、commit）
- [x] 说明回滚后这些修复的当前生效状态

## Findings

- 发布正文（`gh release view 0.0.18 -R zhemed/komari --json body`）与本机保存的
  `.build/rel-notes-0.0.18.md` **逐字节一致**（仅差一个末尾空行）→ 双方口径相同，无二义。
- 标签 `0.0.18` 指向 `4479580`（09-18 14:46），代码提交为 `dd0486a`（14:33）；
  release 发布于 2026-09-18T14:47:59Z，18 资产，非 prerelease。
- 修复 1（host 网络下识别不了自身容器）：
  - 0.0.17 代码实证：`internal/upgrade/docker.go` L32-L35 里 `DetectSelfContainerID()` 返回空即报错；
    `internal/upgrade/upgrade.go` L181-L196 因此拿不到 `ModeDockerRecreate`，落到
    `ModeContainerReplace`（容器内换二进制，**image 不变**）。
  - 后果链：升级当次成功，但下次 `up -d` 或重建容器会回到 compose 文件里的旧 tag。
  - 0.0.18 新增第三条线索：`/proc/self/mountinfo` 的 bind 挂载宿主路径与容器 `Mounts` 比对。
- 修复 2（`ListContainers` 路径拼错）：整条 URL 又传进内部还会拼 `/v<api>` 的函数，请求变成
  `http://docker/v1.41http://docker/...` → 404，让上面那条新线索永远匹配不上。
- 状态：这两条修复随全面回滚被删除（`internal/dockerapi/selfid.go` 等），当前仓库与生产
  `/opt/komari/komari` 均为 0.0.17，host 网络 compose 部署再次只能走容器内替换。

## Notes

- 本次只读，未改任何文件、未碰发布物、未碰生产。
