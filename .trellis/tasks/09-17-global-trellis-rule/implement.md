# 执行计划

1. 在 `~/.dsh/AGENTS.md` 的 `<!-- TRELLIS-GLOBAL:END -->` 之后插入 `TRELLIS-MANDATORY` 段
   （五条：建任务 / 提交锚点 / 收尾留痕 / 项目闸门优先 / 诚实边界 + 参考实现指引）。
2. 读回文件核对：两个原托管块字节未变，新段在块外。
3. komari `.trellis/spec/guides/trellis-gate-guide.md` 增一节"全局推广"，提交带
   `[task:global-trellis-rule]`。
4. `./scripts/check-trellis-gate.sh` + `./scripts/check-repo.sh`，push，看 CI。
5. journal + `finish` + `archive`。
