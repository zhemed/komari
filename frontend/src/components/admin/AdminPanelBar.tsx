import { Cross1Icon, ExitIcon } from "@radix-ui/react-icons";
import {
  Button,
  Flex,
  Grid,
  IconButton,
  Text,
} from "@radix-ui/themes";
import { AnimatePresence, motion } from "framer-motion"; // 引入 Framer Motion
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation /*useNavigate*/ } from "react-router-dom";
import ColorSwitch from "../ColorSwitch";
import LanguageSwitch from "../Language";
import ThemeSwitch from "../ThemeSwitch";
import { useIsMobile } from "@/hooks/use-mobile";
import menuConfig from "../../config/menuConfig.json";
import type { MenuItem } from "../../types/menu";
import { iconMap } from "../../utils/iconHelper";
import { ChevronDownIcon } from "@radix-ui/react-icons";
import { TablerMenu2 } from "../Icones/Tabler";
import LoginDialog from "../Login";
import InlineSvgIcon from "../InlineSvgIcon";
import { useAdminNavigation } from "@/contexts/AdminNavigationContext";
import { useAccount } from "@/contexts/AccountContext";
import { usePublicInfo } from "@/contexts/PublicInfoContext";
import Tips from "../ui/tips";
import { CircleFadingArrowUp } from "lucide-react";
import { useRPC2Call } from "@/contexts/RPC2Context";
import { resolveI18nText } from "@/utils/i18nText";
import {
  getThemeConfigurationType,
  normalizeThemeRedirectTarget,
  THEME_CONFIGURATION_MANAGED,
  THEME_CONFIGURATION_RAW,
  THEME_CONFIGURATION_REDIRECT,
} from "@/utils/themeConfiguration";

// 将JSON配置转换为类型安全的菜单项数组 (基础静态菜单)
const baseMenuItems = (menuConfig as { menu: MenuItem[] }).menu;

// 更新检查目标仓库：默认指向本 fork 的自有仓库，
// 可在构建期用 VITE_KOMARI_UPDATE_REPO=owner/repo 覆盖。
const UPDATE_REPO: string =
  (import.meta.env.VITE_KOMARI_UPDATE_REPO as string | undefined)?.trim() ||
  "zhemed/komari";

// 扩展的菜单项类型（允许直接提供 rawLabel 而不是多语言 key）
interface ExtendedMenuItem extends MenuItem {
  rawLabel?: string; // 不走 i18n，直接显示
  reloadDocument?: boolean;
}

interface AdminPanelBarProps {
  content: ReactNode;
}

const AdminPanelBar = ({ content }: AdminPanelBarProps) => {
  const { call } = useRPC2Call();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [openSubMenus, setOpenSubMenus] = useState<{ [key: string]: boolean }>({
    // 默认所有子菜单关闭
  });
  const { account } = useAccount();
  const isMobile = useIsMobile();
  const [t, i18n] = useTranslation();
  const location = useLocation();
  const isConfigFormPage = location.pathname === "/admin/theme_managed";
  const { publicInfo } = usePublicInfo();
  const { refreshVersion } = useAdminNavigation();
  //const navigate = useNavigate();
  // 获取版本信息
  const [versionInfo, setVersionInfo] = useState<{
    hash: string;
    version: string;
  } | null>(null);
  const currentLanguage =
    i18n.resolvedLanguage ||
    i18n.language ||
    (typeof navigator !== "undefined" ? navigator.language : "");
  // GitHub 最新发布信息与更新检测
  interface GithubReleaseInfo {
    tag_name: string;
    name?: string;
    body?: string;
    html_url: string;
    published_at?: string;
    draft?: boolean;
    prerelease?: boolean;
  }
  const [latestRelease, setLatestRelease] = useState<GithubReleaseInfo | null>(
    null,
  );
  const [updateAvailable, setUpdateAvailable] = useState(false);
  const [releasesSince, setReleasesSince] = useState<GithubReleaseInfo[]>([]);

  // ---- 一键升级（服务端自升级）相关状态 ----
  // 说明：升级动作全部由服务端执行（下载/校验/替换二进制/退出交 systemd 重启），
  // 前端只负责触发与展示进度；容器/无 systemd 等形态由服务端返回 supported=false 或 pull 命令。
  type UpgradePhase =
    | "idle"
    | "downloading"
    | "verifying"
    | "replacing"
    | "restarting"
    | "failed"
    | "completed";
  interface UpgradeStatusInfo {
    phase: UpgradePhase;
    mode?:
      | "binary"
      | "docker-recreate"
      | "container-replace"
      | "manual"
      | "download-only";
    detail?: string;
    from?: string;
    to?: string;
    error?: string;
    backup_path?: string;
    updates_dir?: string;
    running?: boolean;
    current_version?: string;
    enabled?: boolean;
    supported?: boolean;
    platform?: string;
    repo?: string;
  }
  const [upgradeStatus, setUpgradeStatus] = useState<UpgradeStatusInfo | null>(
    null,
  );
  const [upgradeNote, setUpgradeNote] = useState("");
  const [pullCommand, setPullCommand] = useState("");

  // 是否由服务端自动完成升级（二进制替换或重建容器）；manual/download-only 只能给命令或只下载。
  const canAutoUpgrade = !!upgradeStatus?.supported;
  const isDockerRecreate = upgradeStatus?.mode === "docker-recreate";
  const isContainerReplace = upgradeStatus?.mode === "container-replace";
  const autoUpgradeLabel = (version: string) =>
    isDockerRecreate
      ? t("upgrade.upgrade_now_container", "立即升级（重建容器）到 {{version}}", {
          version,
        })
      : isContainerReplace
        ? t(
            "upgrade.upgrade_now_inplace",
            "立即升级（容器内替换）到 {{version}}",
            { version },
          )
        : t("upgrade.upgrade_now", "立即升级到 {{version}}", { version });

  const upgradePhaseLabel = (phase?: UpgradePhase) => {
    switch (phase) {
      case "downloading":
        return t("upgrade.phase_downloading", "下载新版本中…");
      case "verifying":
        return t("upgrade.phase_verifying", "校验并自检中…");
      case "replacing":
        return isDockerRecreate
          ? t("upgrade.phase_recreating", "正在重建容器…")
          : t("upgrade.phase_replacing", "替换二进制中…");
      case "restarting":
        return isContainerReplace
          ? t("upgrade.phase_inplace_restart", "正在容器内原地重启，等待新版本上线…")
          : t("upgrade.phase_restarting", "正在重启服务，等待新版本上线…");
      case "failed":
        return t("upgrade.phase_failed", "升级失败");
      case "completed":
        return t("upgrade.phase_completed", "已就绪");
      default:
        return "";
    }
  };

  // 轮询升级状态；服务重启会让请求失败，此时按“重启中”继续等版本号变化。
  const pollUpgrade = (targetTag?: string) => {
    const startedAt = Date.now();
    const tick = async () => {
      if (Date.now() - startedAt > 5 * 60 * 1000) {
        setUpgradeNote(
          t("upgrade.timeout", "升级状态轮询超时，请刷新页面确认版本"),
        );
        return;
      }
      try {
        const st = await call<any, UpgradeStatusInfo>("admin:upgradeStatus", {});
        setUpgradeStatus(st);
        if (st.phase === "failed") {
          setUpgradeNote(st.error || t("upgrade.phase_failed", "升级失败"));
          return;
        }
        const current = (publicInfo as any)?.version || versionInfo?.version;
        if (targetTag && st.current_version === targetTag && current !== targetTag) {
          setUpgradeNote(t("upgrade.done", "升级完成，页面即将刷新"));
          setTimeout(() => window.location.reload(), 1500);
          return;
        }
        setUpgradeNote(upgradePhaseLabel(st.phase));
        setTimeout(tick, 1000);
      } catch {
        setUpgradeNote(
          t("upgrade.phase_restarting", "正在重启服务，等待新版本上线…"),
        );
        fetch("/api/version", { cache: "no-store" })
          .then((r) => r.json())
          .then((d) => {
            const v = d?.data?.version;
            if (targetTag && v === targetTag) {
              setUpgradeNote(t("upgrade.done", "升级完成，页面即将刷新"));
              setTimeout(() => window.location.reload(), 1000);
              return;
            }
          })
          .catch(() => undefined);
        setTimeout(tick, 2000);
      }
    };
    tick();
  };

  const startUpgrade = async (tag?: string) => {
    setPullCommand("");
    setUpgradeNote(t("upgrade.starting", "正在开始升级…"));
    try {
      const res = await call<
        { tag?: string },
        {
          started?: boolean;
          manual?: boolean;
          pull_command?: string;
          message?: string;
          to?: string;
          from?: string;
        }
      >("admin:upgradeServer", tag ? { tag } : {});
      if (res?.manual) {
        setPullCommand(res.pull_command || "");
        setUpgradeNote(
          res.message || t("upgrade.hint_container", "容器内请拉取新镜像后重建容器"),
        );
        return;
      }
      setUpgradeNote(t("upgrade.started", "升级已开始：下载并校验新版本…"));
      pollUpgrade(res?.to || tag);
    } catch (e: any) {
      setUpgradeNote(e?.message || String(e));
    }
  };

  // 打开有新版提示时先取一次状态（是否支持、开关是否关闭、上次结果）
  useEffect(() => {
    if (!updateAvailable) return;
    let ignore = false;
    call<any, UpgradeStatusInfo>("admin:upgradeStatus", {})
      .then((st) => {
        if (!ignore) setUpgradeStatus(st);
      })
      .catch(() => undefined);
    return () => {
      ignore = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [updateAvailable]);

  const currentTheme = publicInfo?.theme;

  // 动态扩展菜单（主题注入页面）
  const [extraMenuItems, setExtraMenuItems] = useState<ExtendedMenuItem[]>([]);

  useEffect(() => {
    let ignore = false;
    async function loadThemeMenu() {
      // 仅当 theme 存在且不等于 default 时扩展
      if (!currentTheme) {
        setExtraMenuItems([]);
        return;
      }
      try {
        const resp = await fetch(`/themes/${currentTheme}/komari-theme.json`, {
          cache: "no-cache",
        });
        if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
        const data = await resp.json();
        if (ignore) return;
        const cfg = data?.configuration;
        if (!cfg) {
          setExtraMenuItems([]);
          return;
        }

        const cfgType = getThemeConfigurationType(cfg);
        let itemPath: string | null = null;
        if (
          cfgType === THEME_CONFIGURATION_MANAGED &&
          Array.isArray(cfg.data) &&
          cfg.data.length > 0
        ) {
          itemPath = "/admin/theme_managed";
        } else if (cfgType === THEME_CONFIGURATION_RAW) {
          itemPath = "/admin/theme_raw";
        } else if (cfgType === THEME_CONFIGURATION_REDIRECT) {
          itemPath = normalizeThemeRedirectTarget(cfg.data);
        }

        if (!itemPath) {
          setExtraMenuItems([]);
          return;
        }
        const rawLabel: string =
          resolveI18nText(cfg.name, currentLanguage) ??
          t("theme.manage_with_name", {
            name: currentTheme === "default" ? "" : currentTheme,
          });
        const icon: string = cfg.icon || "Palette"; // fallback icon
        const item: ExtendedMenuItem = {
          labelKey: rawLabel,
          rawLabel,
          path: itemPath,
          icon,
          reloadDocument: cfgType === THEME_CONFIGURATION_REDIRECT,
        };
        setExtraMenuItems([item]);
      } catch (e) {
        console.warn("加载主题配置失败，将不扩展主题菜单:", e);
        if (!ignore) setExtraMenuItems([]);
      }
    }
    loadThemeMenu();
    return () => {
      ignore = true;
    };
  }, [currentTheme, refreshVersion]);
  useEffect(() => {
    const fetchVersionInfo = async () => {
      try {
        //const response = await fetch("/api/version");
        const data = await call("common:getVersion");
        setVersionInfo({
          hash: data.hash?.slice(0, 7),
          version: data.version,
        });
      } catch (error) {
        console.error("Failed to fetch version info:", error);
      }
    };

    fetchVersionInfo();
  }, []);

  // 规范化版本为 [major, minor, patch] 数组，忽略前缀 v 和后缀
  function parseSemver(input?: string | null): number[] | null {
    if (!input) return null;
    const s = String(input).trim().replace(/^v/i, "");
    const match = s.match(/^(\d+)\.(\d+)\.(\d+)/);
    if (!match) return null;
    return [Number(match[1]), Number(match[2]), Number(match[3])];
  }

  function isNewerVersion(latest?: string | null, current?: string | null) {
    const a = parseSemver(latest);
    const b = parseSemver(current);
    if (!a || !b) return false;
    for (let i = 0; i < 3; i++) {
      if (a[i] > b[i]) return true;
      if (a[i] < b[i]) return false;
    }
    return false;
  }

  // 获取 GitHub releases 列表，并筛选出“比当前版本新的所有 release”
  useEffect(() => {
    let ignore = false;
    const currentVersion = (publicInfo as any)?.version || versionInfo?.version;
    if (!currentVersion) return;

    async function loadReleases() {
      try {
        const resp = await fetch(
          `https://api.github.com/repos/${UPDATE_REPO}/releases?per_page=100`,
          {
            headers: {
              Accept: "application/vnd.github+json",
            },
            cache: "no-cache",
          },
        );
        if (!resp.ok) throw new Error(`GitHub HTTP ${resp.status}`);
        const data: GithubReleaseInfo[] = await resp.json();
        if (ignore) return;
        const valid = (data || [])
          .filter((r) => !r.draft && !r.prerelease)
          .filter((r) =>
            isNewerVersion(r?.tag_name || r?.name, currentVersion),
          );
        setReleasesSince(valid);
        setLatestRelease(valid.length ? valid[0] : null);
        setUpdateAvailable(valid.length > 0);
      } catch (e) {
        console.warn("加载 GitHub 最新发布失败:", e);
        if (!ignore) {
          setLatestRelease(null);
          setReleasesSince([]);
          setUpdateAvailable(false);
        }
      }
    }

    loadReleases();
    return () => {
      ignore = true;
    };
  }, [publicInfo, versionInfo]);
  // Handle responsive behavior
  useEffect(() => {
    const handleResize = () => setSidebarOpen(!isMobile);
    handleResize();
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, [isMobile]);

  // 主题配置和插件注入页面分别作为“主题”“插件”主菜单的二级菜单。
  const mergedBaseMenuItems: ExtendedMenuItem[] = useMemo(() => {
    return baseMenuItems.map((item) => {
      if (item.labelKey === "theme.menu" && extraMenuItems.length > 0) {
        return {
          ...item,
          children: [...(item.children || []), ...extraMenuItems],
        };
      }
      return item;
    });
  }, [extraMenuItems]);
  const bottomStartPath = mergedBaseMenuItems.find(
    (item) => item.bottom,
  )?.path;

  // 根据路径自动展开子菜单（动态扩展项可能带 query，故子菜单匹配基于 pathname 部分）
  useEffect(() => {
    const newState: { [key: string]: boolean } = {};
    const combined: ExtendedMenuItem[] = mergedBaseMenuItems;
    combined.forEach((item) => {
      if (item.children) {
        newState[item.path] = item.children.some((child: MenuItem) => {
          const childPath = child.path.split("?")[0];
          return (
            location.pathname === childPath ||
            (childPath !== "/" &&
              location.pathname.startsWith(childPath + "/"))
          );
        });
      }
    });
    setOpenSubMenus(newState);
  }, [location.pathname, extraMenuItems, mergedBaseMenuItems]);

  // 侧边栏动画变体
  const sidebarVariants = {
    open: {
      width: isMobile ? "100vw" : "240px",
      opacity: 1,
      transition: {
        type: "spring",
        stiffness: 300,
        damping: 30,
      },
    },
    closed: {
      width: 0,
      opacity: isMobile ? 0 : 1, // 移动端完全透明
      transition: {
        type: "spring",
        stiffness: 300,
        damping: 30,
      },
    },
  };

  // 内容区域动画变体
  const contentVariants = {
    open: {
      opacity: isMobile ? 0 : 1,
      x: isMobile ? "100%" : 0,
      transition: {
        duration: 0.3,
      },
    },
    closed: {
      opacity: 1,
      x: 0,
      transition: {
        duration: 0.3,
      },
    },
  };

  function logout() {
    window.open("/api/logout", "_self");
  }
  return (
    <>
      <Grid
        className="km-admin-layout km-admin-panel-bar"
        columns={{ initial: "1fr", md: sidebarOpen ? "240px 1fr" : "0px 1fr" }} // 动态调整网格列
        rows={{ initial: "auto 1fr", md: "auto 1fr" }}
        style={{
          height: "100vh",
          width: "100vw",
          overflow: "hidden",
          backgroundColor: "var(--accent-1)",
        }}
      >
        {/* Navbar */}
        <motion.nav
          className="km-admin-panel-topbar col-span-2"
          initial={{ y: 0 }}
          animate={{ y: 0 }}
          transition={{ duration: 0.5, ease: "easeOut" }}
        >
          <Flex
            gap="3"
            p="2"
            justify="between"
            align="center"
            className="border-b-1"
          >
            <Flex gap="3" align="center">
              <IconButton
                variant="ghost"
                onClick={() => setSidebarOpen(!sidebarOpen)}
                title={t("common.menu_sidebar", "Menu")}
                aria-label={t("common.menu_sidebar", "Menu")}
                style={{
                  display: isMobile && sidebarOpen ? "none" : "flex",
                  color: "var(--gray-11)",
                }}
              >
                <TablerMenu2 />
              </IconButton>
              <a href="/" target="_blank" rel="noopener noreferrer">
                <label className="text-xl font-bold">Komari</label>
              </a>
              {updateAvailable && releasesSince.length > 0 && (
                <Tips
                  mode="dialog"
                  className="check-update"
                  trigger={<CircleFadingArrowUp color="#FB4141" size="16" />}
                >
                  <div className="flex flex-col gap-2 max-w-[80vw] md:max-w-[720px]">
                    <label className="font-bold">
                      {t("common.update_available")}
                    </label>
                    <div className="text-sm text-muted-foreground">
                      <span style={{ marginRight: 8 }}>
                        {(publicInfo as any)?.version || versionInfo?.version}
                      </span>
                      <span>{"> "}</span>
                      <span>
                        {(latestRelease?.tag_name || latestRelease?.name) ?? ""}
                      </span>
                    </div>

                    <div className="rounded-md p-2 overflow-auto max-h-80">
                      <div className="flex flex-col gap-4 text-sm">
                        {releasesSince.map((r) => (
                          <div key={r.html_url} className="flex flex-col gap-2">
                            <div className="flex items-center justify-between">
                              <div className="font-medium">
                                {r.name || r.tag_name}
                              </div>
                              {r.published_at && (
                                <div className="text-xs text-muted-foreground">
                                  {new Date(r.published_at).toLocaleString()}
                                </div>
                              )}
                            </div>
                            <div className="whitespace-pre-wrap break-words">
                              {r.body || ""}
                            </div>
                            <div
                              style={{
                                height: 1,
                                background: "var(--accent-5)",
                                opacity: 0.5,
                              }}
                            />
                            {/* 能一键升级 → 升级按钮；不能（容器/无 systemd）→ 给可复制的命令入口。
                                两者的区别只是服务端会不会真的替换二进制，前端入口都要有：
                                0.0.9 的缺口就是"不支持"时把入口一起藏了，用户拿不到命令。 */}
                            {upgradeStatus && upgradeStatus.enabled !== false && (
                              <div className="flex justify-end">
                                <Button
                                  size="1"
                                  variant="soft"
                                  disabled={!!upgradeStatus?.running}
                                  onClick={() =>
                                    startUpgrade(r.tag_name || r.name)
                                  }
                                >
                                  {canAutoUpgrade
                                    ? t("upgrade.install_version", "安装此版本")
                                    : t(
                                        "upgrade.copy_pull_command",
                                        "复制升级命令",
                                      )}
                                </Button>
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                    {/* 一键升级：状态、拉取命令与出错信息 */}
                    {upgradeStatus?.enabled === false && (
                      <div className="text-xs text-muted-foreground">
                        {t(
                          "upgrade.disabled",
                          "一键升级已在设置中关闭（可在系统设置里打开）",
                        )}
                      </div>
                    )}
                    {upgradeStatus &&
                      upgradeStatus.enabled !== false &&
                      !upgradeStatus.supported && (
                        <div className="text-xs text-muted-foreground">
                          {t(
                            "upgrade.unsupported",
                            "当前部署形态不支持一键升级（容器请拉镜像，前台运行请手工替换）",
                          )}
                          {upgradeStatus.platform
                            ? ` · ${upgradeStatus.platform}`
                            : ""}
                        </div>
                      )}
                    {isDockerRecreate && (
                      <div className="text-xs text-muted-foreground">
                        {t(
                          "upgrade.docker_socket_hint",
                          "该模式通过挂载的 Docker socket 拉取镜像并重建容器——等于把宿主控制权交给本容器，请确认这是你想要的。",
                        )}
                      </div>
                    )}
                    {isContainerReplace && (
                      <div className="text-xs text-muted-foreground">
                        {t(
                          "upgrade.inplace_hint",
                          "容器内替换二进制并原地重启：不需要挂载、不需要额外配置；但之后重建容器（docker rm + run / compose up）会退回镜像里的版本——想与镜像完全一致，请挂 /var/run/docker.sock 使用重建容器模式。",
                        )}
                      </div>
                    )}
                    {upgradeNote && (
                      <div className="text-xs text-muted-foreground">
                        {upgradeNote}
                      </div>
                    )}
                    {pullCommand && (
                      <div className="flex items-center gap-2">
                        <code className="text-xs break-all">{pullCommand}</code>
                        <Button
                          size="1"
                          variant="soft"
                          onClick={() =>
                            navigator.clipboard?.writeText(pullCommand)
                          }
                        >
                          {t("upgrade.copy_command", "复制命令")}
                        </Button>
                      </div>
                    )}
                    <div className="flex justify-end gap-2">
                      {upgradeStatus && upgradeStatus.enabled !== false && latestRelease && (
                        <Button
                          variant={canAutoUpgrade ? undefined : "soft"}
                          disabled={!!upgradeStatus?.running}
                          onClick={() =>
                            startUpgrade(
                              latestRelease?.tag_name || latestRelease?.name,
                            )
                          }
                        >
                          {canAutoUpgrade
                            ? autoUpgradeLabel(
                                latestRelease?.tag_name ||
                                  latestRelease?.name ||
                                  "",
                              )
                            : t(
                                "upgrade.copy_pull_command",
                                "复制升级命令",
                              )}
                        </Button>
                      )}
                      <a
                        href={latestRelease?.html_url}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        <Button variant="soft">Github</Button>
                      </a>
                    </div>
                  </div>
                </Tips>
              )}
              <label
                className="text-sm text-muted-foreground self-end overflow-hidden"
                hidden={isMobile}
              >
                {(publicInfo as any)?.version ||
                  (versionInfo &&
                    `${versionInfo.version} (${versionInfo.hash})`)}
              </label>
            </Flex>
            <Flex gap="3" align="center" overflowX="auto" className="km-admin-panel-controls">
              {account && !account.logged_in && (
                <LoginDialog
                  autoOpen={true}
                  showSettings={false}
                  onLoginSuccess={() => {
                    window.location.reload();
                  }}
                />
              )}
              <ThemeSwitch />
              <ColorSwitch />
              <LanguageSwitch />
              <IconButton
                variant="soft"
                color="orange"
                className="km-admin-panel-account"
                onClick={logout}
                title={t("common.logout", "Logout")}
                aria-label={t("common.logout", "Logout")}
              >
                <ExitIcon />
              </IconButton>
            </Flex>
          </Flex>
        </motion.nav>

        {/* Sidebar */}
        <AnimatePresence>
          <motion.div
            variants={sidebarVariants}
            initial="closed"
            animate={sidebarOpen ? "open" : "closed"}
            exit="closed"
            className="km-admin-panel-nav"
            style={{
              backgroundColor: "var(--accent-1)",
              height: "100%",
              position: isMobile ? "absolute" : "relative",
              zIndex: isMobile ? 10 : 1,
              overflowY: "auto",
              overflowX: "hidden",
            }}
          >
            <Flex
              gap="3"
              className="p-2 border-r-1"
              direction="column"
              justify="start"
              align="start"
              style={{ height: "100%", minWidth: "240px" }}
            >
              {/* 关闭按钮 */}
              <IconButton
                variant="soft"
                title={t("common.close_sidebar", "Close menu")}
                aria-label={t("common.close_sidebar", "Close menu")}
                style={{
                  display: isMobile ? "flex" : "none",
                  margin: "8px 0px 0px 8px",
                }}
                onClick={() => setSidebarOpen(false)}
              >
                <Cross1Icon />
              </IconButton>
              {/* 侧边连链接 */}
              <Flex
                direction="column"
                gap="1"
                className="h-full md:mt-0 mt-6"
                style={{ width: "100%" }}
              >
                {mergedBaseMenuItems.map(
                  (item: ExtendedMenuItem) => {
                    // 支持 icon 为 URL/相对路径
                    const isOpen = openSubMenus[item.path];
                    const renderIcon = (
                      icon: string,
                      labelKey: string,
                      className?: string,
                      active?: boolean,
                    ) => {
                      const link = /^(https?:\/\/|\/|\.\/|\.\.\/)/.test(icon);
                      if (link) {
                        return (
                          <InlineSvgIcon
                            src={icon}
                            alt={t(labelKey)}
                            style={{
                              width: 16,
                              height: 16,
                              objectFit: "contain",
                              opacity: active ? 1 : 0.7,
                              filter: active ? "none" : "grayscale(20%)",
                            }}
                            className={className}
                            loading="lazy"
                          />
                        );
                      }
                      const Cmp = iconMap[icon];
                      if (Cmp) {
                        return (
                          <Cmp
                            className={className}
                            style={{
                              color: active
                                ? "var(--accent-10)"
                                : "var(--gray11)",
                            }}
                          />
                        );
                      }
                      // fallback: simple dot
                      return (
                        <span
                          className={className}
                          style={{
                            width: 16,
                            height: 16,
                            display: "inline-block",
                            borderRadius: 4,
                            background: "var(--accent-8)",
                          }}
                        />
                      );
                    };
                    if (item.children && item.children.length) {
                      return (
                        <div key={item.path}>
                          <Flex
                            className="p-2 gap-2 border-l-[4px] border-transparent cursor-pointer hover:bg-accent-3 rounded-md"
                            align="center"
                            onClick={() => {
                              //const currentlyOpen = openSubMenus[item.path];
                              // 检查当前路径是否已经在该父菜单的子菜单中
                              //const isCurrentlyInThisMenu = item.children?.some(
                              //  (child) =>
                              //    location.pathname === child.path ||
                              //    location.pathname.startsWith(child.path)
                              //);

                              // 切换子菜单的展开状态
                              setOpenSubMenus((prev) => ({
                                ...prev,
                                [item.path]: !prev[item.path],
                              }));

                              //// 只有在非展开状态且不在当前菜单组中时才导航到第一个子菜单项
                              //if (
                              //  !currentlyOpen &&
                              //  !isCurrentlyInThisMenu &&
                              //  item.children &&
                              //  item.children.length > 0
                              //) {
                              //  //navigate(item.children[0].path);
                              //  // 如果是移动端，关闭侧边栏
                              //  if (isMobile) {
                              //    setSidebarOpen(false);
                              //  }
                              //}
                            }}
                          >
                            {renderIcon(
                              item.icon,
                              item.labelKey,
                              "flex w-4 h-5 items-center justify-center",
                            )}
                            <Text
                              className="text-base"
                              weight="medium"
                              style={{
                                flex: 1,
                              }}
                            >
                              {item.rawLabel || t(item.labelKey)}
                            </Text>

                            <ChevronDownIcon
                              style={{
                                transform: isOpen
                                  ? "rotate(180deg)"
                                  : "rotate(0deg)",
                                transition: "transform 0.2s",
                              }}
                            />
                          </Flex>
                          <motion.div
                            initial={{ height: 0, opacity: 0 }}
                            animate={
                              isOpen
                                ? { height: "auto", opacity: 1 }
                                : { height: 0, opacity: 0 }
                            }
                            transition={{ duration: 0.2 }}
                            style={{ overflow: "hidden" }}
                          >
                            <Flex direction="column" className="ml-4 gap-1">
                              {item.children.map((child: MenuItem) => (
                                <SidebarItem
                                  key={child.path}
                                  to={child.path}
                                  icon={renderIcon(
                                    child.icon,
                                    child.labelKey,
                                    "flex w-4 h-5 items-center justify-center",
                                  )}
                                  children={
                                    (child as ExtendedMenuItem).rawLabel ||
                                    t(child.labelKey)
                                  }
                                  onClick={() =>
                                    isMobile && setSidebarOpen(false)
                                  }
                                  newTab={child.newTab}
                                  reloadDocument={
                                    (child as ExtendedMenuItem).reloadDocument
                                  }
                                />
                              ))}
                            </Flex>
                          </motion.div>
                        </div>
                      );
                    }
                    const isBottomStart =
                      item.bottom && item.path === bottomStartPath;
                    return (
                      <div
                        key={item.path}
                        style={
                          isBottomStart
                            ? { marginTop: "auto" }
                            : undefined
                        }
                      >
                        <SidebarItem
                          to={item.path}
                          icon={renderIcon(
                            item.icon,
                            item.labelKey,
                            "flex w-4 h-5 items-center justify-center",
                          )}
                          children={item.rawLabel || t(item.labelKey)}
                          onClick={() => isMobile && setSidebarOpen(false)}
                          newTab={item.newTab}
                          reloadDocument={item.reloadDocument}
                        />
                      </div>
                    );
                  },
                )}
              </Flex>
            </Flex>
          </motion.div>
        </AnimatePresence>

        {/* Main Content */}
        <motion.div
          variants={contentVariants}
          animate={sidebarOpen ? "open" : "closed"}
          className="km-admin-panel-content"
          style={{
            backgroundColor: "var(--accent-3)",
            display: isMobile && sidebarOpen ? "none" : "block",
            height: "100%", // Ensure the container takes full height
            minHeight: 0,
            overflow: "hidden", // Prevent this container from scrolling
          }}
        >
          <div
            style={{
              backgroundColor: "var(--accent-1)",
              height: "100%",
              minHeight: 0,
              borderRadius: "0",
              padding: isMobile ? "8px" : "16px",
              overflowY: isConfigFormPage ? "hidden" : "auto",
              display: isConfigFormPage ? "flex" : "block",
              flexDirection: isConfigFormPage ? "column" : undefined,
              boxSizing: "border-box",
            }}
          >
            {isConfigFormPage ? (
              <div className="min-h-0 flex-1">{content}</div>
            ) : (
              content
            )}
          </div>
        </motion.div>
      </Grid>
    </>
  );
};

export default AdminPanelBar;

// 侧边栏项目组件
const SidebarItem = ({
  to,
  onClick,
  icon,
  children,
  newTab,
  reloadDocument,
}: {
  to: string;
  onClick: () => void;
  icon: ReactNode;
  children: ReactNode;
  newTab?: boolean;
  reloadDocument?: boolean;
}) => {
  const location = useLocation();
  const isExternalLink = to.startsWith("http://") || to.startsWith("https://");
  // 带 query 的菜单项做全匹配；不带 query 的菜单项只比 pathname，
  // 同时避免前缀兄弟路由（/admin/x 与 /admin/x/y）同时点亮。
  const isActive =
    !isExternalLink &&
    to !== "/" &&
    (to.includes("?")
      ? location.pathname + location.search === to
      : location.pathname === to.split("?")[0]);
  const openInNewTab = newTab === true || (isExternalLink && newTab !== false);

  if (openInNewTab || reloadDocument) {
    return (
      <a
        href={to}
        onClick={onClick}
        target={openInNewTab ? "_blank" : undefined}
        rel={openInNewTab ? "noopener noreferrer" : undefined}
        className="group transition-colors duration-200 hover:bg-accent-3 rounded-md"
      >
        <Flex
          className="p-2 gap-2 h-full"
          align="center"
          style={{
            borderLeft: "4px solid transparent",
            borderRadius: "6px",
            backgroundColor: "transparent",
            color: "inherit",
            transition: "background-color 0.2s, border-color 0.2s",
          }}
        >
          <span
            style={{
              color: "inherit",
              opacity: 0.7,
            }}
            className="flex w-4 h-5 items-center justify-center"
          >
            {icon}
          </span>
          <Text className="text-base" weight="medium" style={{ flex: 1 }}>
            {children}
          </Text>
        </Flex>
      </a>
    );
  }

  return (
    <Link
      to={to}
      onClick={onClick}
      className="group transition-colors duration-200 hover:bg-accent-3 rounded-md"
    >
      <Flex
        className="p-2 gap-2"
        align="center"
        style={{
          borderLeft: isActive
            ? "4px solid var(--accent-8)"
            : "4px solid transparent",
          borderRadius: "6px",
          backgroundColor: isActive ? "var(--accent-4)" : "transparent",
          color: isActive ? "var(--accent-10)" : "inherit",
          transition: "background-color 0.2s, border-color 0.2s",
        }}
      >
        <span
          style={{
            color: isActive ? "var(--accent-10)" : "inherit",
            opacity: isActive ? 1 : 0.7,
          }}
          className="flex w-4 h-5 items-center justify-center"
        >
          {icon}
        </span>
        <Text
          className="text-base"
          weight={isActive ? "bold" : "medium"}
          style={{ flex: 1 }}
        >
          {children}
        </Text>
      </Flex>
    </Link>
  );
};
