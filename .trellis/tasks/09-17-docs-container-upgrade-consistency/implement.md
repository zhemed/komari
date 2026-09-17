# 执行计划：文档一致性（容器升级口径）

口径唯一版本（所有落点照抄这三条）：

1. 容器（默认，不挂任何东西）→ ✅ 面板一键升级：容器内下载 → 校验 → 自检 → 原子替换 → 原地重执行；
2. 容器 + 挂 `/var/run/docker.sock`（**可选**）→ 改为"拉镜像 + helper 重建容器"，版本与镜像完全一致，
   代价是宿主 root 等价权限；
3. 不挂 socket 时，用 `docker rm + run` / `compose up` **重建容器**会把二进制退回镜像版本；
   `docker restart` 不会升级。

## Step 1 · README
- [ ] L56-61 命令块删掉 `-v /var/run/docker.sock:/var/run/docker.sock \` 行
- [ ] L65-69 三条说明保留并压成同一口径（开箱可用 / 可选增强与代价 / 重建容器会回退）
- [ ] L104-107 功能点：把"没挂 socket 时只给可复制的 pull 命令"改成容器内替换 + 不挂 socket 的代价

## Step 2 · docs/MAINTAINING.md
- [ ] §14.2（L592-598）表格行：`容器（默认，不挂 socket）✅ 容器内替换` / `容器 + 挂 socket ✅ 重建容器（可选）`
      两行都写"能否升级 = ✅"，把差别落在"版本与镜像是否一致"
- [ ] §14.6 标题去掉"挂 docker socket 时"，改成中性的"容器一键升级（0.0.11 起…）"
- [ ] §14.4.2 示例注释改为"想用重建容器模式时加这一行（可选）"
- [ ] §14.6 正文 L662 那句改为"没挂 socket 时走容器内替换，systemd 形态不受影响"

## Step 3 · 复核（证据）
- [ ] `grep -n "docker.sock\|没挂 socket\|必须挂" README.md docs/MAINTAINING.md` 逐条人工确认
- [ ] `./scripts/check-repo.sh`
- [ ] commit（msg 用文件传入，避免 ASCII 引号）+ journal + `task.py finish/archive`
