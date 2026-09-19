# 定稿唯一部署路径：install-komari.sh（systemd）

## 用户指令

2026-09-19：「验收成功，我们还是用这样的部署命令吧，别用其他的了」。

读法：`install-komari.sh`（systemd）是**唯一**部署/升级路径；不再引入第二套部署方式。
这不是"容器不能用"，而是"仓库里不再维护第二套部署自动化"——上一套（compose）就是这么出事的。

## 做了什么

1. **README**：部署段重排——「一键安装 / 升级（systemd，推荐）」写明是**唯一主推路径**，
   升级明确"再跑一次 + 菜单选 2"；Docker 降为「替代方案」并标注非主推、不许再分叉；
   删掉过时的 `compose up` 表述，补上 host 网络 + socket 那条静默回退的告警。
2. **MAINTAINING §3.4.1（新增）**：唯一路径的定调、验收过的升级步骤
   （`KOMARI_TAG=<版本> bash install-komari.sh` → 菜单 2 → stable）、
   三道验收命令（version/hash、sha256 对比、systemctl + 错误行数）、
   非交互执行的三个坑（TUI 需真 PTY；菜单要方向键；pexpect 中文需 encoding=None 且不收 tuple）、
   回滚方式（换回 `komari.backup.<时间戳>` + `data/backup/upgrade-<时间戳>.zip`）。
3. **机械守卫 `check-repo.sh` 第 14 项**：仓库根带 `deploy-entry:` 标记的脚本只允许三个
   （`install-komari.sh` / `install-agent.sh` / `install-agent.ps1`），出现第二个部署入口即报红；
   并校验 install-komari.sh 的默认 tag = version.env、装到 `/opt/komari/komari`、
   systemd 工作目录 = `${DATA_DIR}`；`--full` 另查 `upgrade_komari()` 存在、TUI 仍是 whiptail/dialog、
   README 与 MAINTAINING 的口径声明还在。
4. **三个入口文件加 `deploy-entry:` 标记注释**（纯注释，行为不变）。

## 过程中抓到的真问题（判别性验证的第二次收获）

第 14 项最初的 `deploy_entries()` 写的是**固定文件名清单**（`install-komari.sh install-agent.sh
install-agent.ps1 scripts`），所以新建的 `install-compose.sh` **根本不在扫描范围内**——守卫永远为绿。
是 `--full` 里的判别性验证（真造一个文件再检查）把它抓出来的。修正为扫描仓库根（排除
`.git`/`node_modules`/`.build`/`dist`）后，实测：造 → 报红，删 → 通过。

## Acceptance Criteria

- [x] README 与 MAINTAINING 口径一致：systemd 一条命令是唯一主推路径
- [x] MAINTAINING §3.4.1 含验收过的步骤、验收清单、驱动要点与回滚方式
- [x] `check-repo.sh` 第 14 项 + 判别性验证（造/删实测）
- [x] `./scripts/check-repo.sh --full` 全绿（14 项）
- [x] 未改任何运行代码；未碰生产与线上发布物
