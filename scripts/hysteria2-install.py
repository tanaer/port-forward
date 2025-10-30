#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Hysteria2 一键安装脚本
版本: 2.0
作者: Integrated & Optimized
描述: Hysteria2代理服务器一键安装、配置和管理工具
"""

import os
import re
import subprocess
import sys
import time
import urllib.request
import urllib.parse
from pathlib import Path

try:
    import requests
except ImportError:
    print("正在安装 requests 模块...")
    subprocess.run([sys.executable, "-m", "pip", "install", "requests"], check=True)
    import requests

# ============================================
# 全局配置
# ============================================
VERSION = "2.0"
SUBSCRIPTION_API = "https://sublink-worker.watrans.workers.dev"
SCRIPT_URL = "https://raw.githubusercontent.com/tanaer/port-forward/master/scripts/hysteria2-install.py"

# 全局变量
hy2_domain = ""
domain_name = ""
insecure = ""

# ============================================
# 系统依赖安装
# ============================================

def install_system_dependencies():
    """安装系统依赖包"""
    print("=" * 60)
    print("检测系统并安装依赖...")
    print("=" * 60)

    # 检测操作系统
    try:
        with open('/etc/os-release', 'r') as f:
            os_info = f.read()

        if 'ubuntu' in os_info.lower() or 'debian' in os_info.lower():
            return install_debian_dependencies()
        elif 'centos' in os_info.lower() or 'rocky' in os_info.lower() or 'fedora' in os_info.lower():
            return install_redhat_dependencies()
        else:
            print("\033[91m不支持的Linux发行版\033[0m")
            return False
    except Exception as e:
        print(f"\033[91m检测系统失败: {e}\033[0m")
        return False

def install_debian_dependencies():
    """安装Debian/Ubuntu依赖"""
    packages = [
        'curl', 'sudo', 'openssl', 'qrencode', 'net-tools',
        'procps', 'iptables', 'ca-certificates', 'python3', 'python3-pip'
    ]

    print("正在更新软件包列表...")
    subprocess.run(["apt", "update"], check=False)

    print("正在安装依赖包...")
    cmd = ["apt", "install", "-y"] + packages
    result = subprocess.run(cmd, check=False)

    if result.returncode == 0:
        print("\033[92m✓ 依赖安装成功\033[0m")
        return True
    else:
        print("\033[91m✗ 依赖安装失败\033[0m")
        return False

def install_redhat_dependencies():
    """安装RedHat/CentOS/Rocky依赖"""
    packages = [
        'curl', 'sudo', 'openssl', 'qrencode', 'net-tools',
        'procps', 'iptables', 'ca-certificates', 'python3', 'python3-pip'
    ]

    print("正在安装EPEL仓库...")
    subprocess.run(["dnf", "install", "-y", "epel-release"], check=False)

    print("正在安装依赖包...")
    cmd = ["dnf", "install", "-y"] + packages
    result = subprocess.run(cmd, check=False)

    if result.returncode == 0:
        print("\033[92m✓ 依赖安装成功\033[0m")
        return True
    else:
        print("\033[91m✗ 依赖安装失败\033[0m")
        return False

# ============================================
# 环境设置
# ============================================

def setup_environment():
    """设置环境和快捷方式"""
    def create_shortcut():
        """创建快捷方式"""
        try:
            hy2_shortcut = Path("/usr/local/bin/hy2")
            shortcut_content = f"""#!/bin/bash
# Hysteria2 快捷启动脚本
curl -fsSL {SCRIPT_URL} -o /tmp/hy2.py && python3 /tmp/hy2.py
"""
            hy2_shortcut.write_text(shortcut_content)
            hy2_shortcut.chmod(0o755)
            print("\033[92m✓ 快捷命令 'hy2' 已创建\033[0m")
        except Exception as e:
            print(f"\033[91m✗ 创建快捷命令失败: {e}\033[0m")

    config_dir = Path("/etc/hy2config")

    try:
        config_dir.mkdir(parents=True, exist_ok=True)
        (config_dir / "hy2_url_scheme.txt").touch()
        create_shortcut()
        return True
    except Exception as e:
        print(f"\033[91m✗ 初始化环境失败: {e}\033[0m")
        return False

# ============================================
# Hysteria2 安装/卸载
# ============================================

def hysteria2_install():
    """安装或更新Hysteria2"""
    while True:
        print("\n" + "=" * 60)
        print("Hysteria2 安装/更新")
        print("=" * 60)
        choice = input("是否安装/更新 Hysteria2？[y/n]: ").strip().lower()

        if choice == "y":
            print("\n1. 安装最新版本")
            print("2. 安装指定版本")
            version_choice = input("\n请选择 [1/2]: ").strip()

            if version_choice == "1":
                print("\n正在安装最新版本...")
                result = subprocess.run(
                    "bash <(curl -fsSL https://get.hy2.sh/)",
                    shell=True,
                    executable="/bin/bash"
                )
                if result.returncode == 0:
                    print("\033[92m✓ Hysteria2 安装成功\033[0m")
                    hysteria2_config()
                else:
                    print("\033[91m✗ Hysteria2 安装失败\033[0m")
                break

            elif version_choice == "2":
                version = input("\n请输入版本号（如 2.6.0）: ").strip()
                print(f"\n正在安装版本 {version}...")
                result = subprocess.run(
                    f"bash <(curl -fsSL https://get.hy2.sh/) --version v{version}",
                    shell=True,
                    executable="/bin/bash"
                )
                if result.returncode == 0:
                    print(f"\033[92m✓ Hysteria2 v{version} 安装成功\033[0m")
                    hysteria2_config()
                else:
                    print(f"\033[91m✗ Hysteria2 v{version} 安装失败\033[0m")
                break
            else:
                print("\033[91m无效选项\033[0m")

        elif choice == "n":
            print("已取消安装")
            break
        else:
            print("\033[91m请输入 y 或 n\033[0m")

def hysteria2_uninstall():
    """卸载Hysteria2"""
    print("\n" + "=" * 60)
    print("Hysteria2 卸载")
    print("=" * 60)
    print("\033[91m警告: 此操作将删除所有配置文件和服务\033[0m")

    while True:
        choice = input("\n确认卸载 Hysteria2？[y/n]: ").strip().lower()

        if choice == "y":
            print("\n正在卸载...")

            # 卸载主程序
            subprocess.run(
                "bash <(curl -fsSL https://get.hy2.sh/) --remove",
                shell=True,
                executable="/bin/bash"
            )

            # 清理配置文件和服务
            cleanup_cmds = [
                "systemctl stop hysteria-server.service 2>/dev/null",
                "systemctl disable hysteria-server.service 2>/dev/null",
                "rm -rf /etc/hysteria",
                "rm -rf /etc/systemd/system/multi-user.target.wants/hysteria-server.service",
                "rm -rf /etc/systemd/system/multi-user.target.wants/hysteria-server@*.service",
                "systemctl daemon-reload",
                "/etc/hy2config/jump_port_back.sh 2>/dev/null",
                "rm -rf /etc/ssl/private/hy2*",
                "rm -rf /etc/hy2config",
                "rm -rf /usr/local/bin/hy2"
            ]

            for cmd in cleanup_cmds:
                subprocess.run(cmd, shell=True, check=False)

            print("\033[92m✓ Hysteria2 卸载完成\033[0m")
            sys.exit(0)

        elif choice == "n":
            print("已取消卸载")
            break
        else:
            print("\033[91m请输入 y 或 n\033[0m")

# ============================================
# 服务管理
# ============================================

def server_manage():
    """Hysteria2服务管理"""
    while True:
        print("\n" + "=" * 60)
        print("Hysteria2 服务管理")
        print("=" * 60)
        print("1. 启动服务（自动开机启动）")
        print("2. 停止服务")
        print("3. 重启服务")
        print("4. 查看服务状态")
        print("5. 查看服务日志")
        print("6. 查看版本信息")
        print("0. 返回主菜单")
        print("=" * 60)

        choice = input("\n请选择 [0-6]: ").strip()

        if choice == "1":
            subprocess.run("systemctl enable --now hysteria-server.service", shell=True)
            print("\033[92m✓ 服务已启动\033[0m")

        elif choice == "2":
            subprocess.run("systemctl stop hysteria-server.service", shell=True)
            print("\033[92m✓ 服务已停止\033[0m")

        elif choice == "3":
            subprocess.run("systemctl restart hysteria-server.service", shell=True)
            print("\033[92m✓ 服务已重启\033[0m")

        elif choice == "4":
            print("\n\033[93m提示: 按 'q' 退出查看\033[0m\n")
            subprocess.run("systemctl status hysteria-server.service", shell=True)

        elif choice == "5":
            print("\n\033[93m提示: 按 'q' 退出查看\033[0m\n")
            subprocess.run("journalctl --no-pager -n 50 -u hysteria-server.service", shell=True)

        elif choice == "6":
            subprocess.run("/usr/local/bin/hysteria version", shell=True)

        elif choice == "0":
            break
        else:
            print("\033[91m无效选项\033[0m")

# ============================================
# IP地址获取
# ============================================

def get_ipv4_address():
    """获取IPv4地址"""
    global hy2_domain
    try:
        response = requests.get('http://ip-api.com/json/', timeout=5)
        response.raise_for_status()
        data = response.json()

        isp = data.get('isp', '').lower()
        if 'cloudflare' in isp:
            ip = input("\n检测到Cloudflare WARP，请手动输入服务器IP: ").strip()
        else:
            ip = data.get('query', '')

        hy2_domain = ip
        print(f"\n\033[92m✓ 服务器IPv4: {hy2_domain}\033[0m")
        return True

    except Exception as e:
        print(f"\033[91m✗ 获取IPv4失败: {e}\033[0m")
        ip = input("请手动输入服务器IPv4: ").strip()
        if ip:
            hy2_domain = ip
            return True
        return False

def get_ipv6_address():
    """获取IPv6地址"""
    global hy2_domain
    try:
        response = requests.get('https://api.ip.sb/geoip', timeout=5)
        response.raise_for_status()
        data = response.json()

        isp = data.get('isp', '').lower()
        if 'cloudflare' in isp:
            ip = input("\n检测到Cloudflare WARP，请手动输入服务器IPv6: ").strip()
        else:
            ip = data.get('ip', '')

        hy2_domain = f"[{ip}]"
        print(f"\n\033[92m✓ 服务器IPv6: {hy2_domain}\033[0m")
        return True

    except Exception as e:
        print(f"\033[91m✗ 获取IPv6失败: {e}\033[0m")
        ip = input("请手动输入服务器IPv6: ").strip()
        if ip:
            hy2_domain = f"[{ip}]"
            return True
        return False

# ============================================
# 证书生成
# ============================================

def generate_self_signed_cert():
    """生成自签名证书"""
    global domain_name

    user_domain = input("\n请输入证书域名（默认 bing.com）: ").strip()
    domain_name = user_domain if user_domain else "bing.com"

    # 验证域名格式
    if not re.match(r'^[a-zA-Z0-9.-]+$', domain_name):
        print("\033[91m✗ 无效的域名格式\033[0m")
        return False

    target_dir = "/etc/ssl/private"

    try:
        # 创建目录
        Path(target_dir).mkdir(parents=True, exist_ok=True)

        # 生成EC参数
        ec_param = f"{target_dir}/ec_param.pem"
        subprocess.run(
            ["openssl", "ecparam", "-name", "prime256v1", "-out", ec_param],
            check=True,
            capture_output=True
        )

        # 生成证书和私钥
        subprocess.run([
            "openssl", "req", "-x509", "-nodes", "-newkey", f"ec:{ec_param}",
            "-keyout", f"{target_dir}/{domain_name}.key",
            "-out", f"{target_dir}/{domain_name}.crt",
            "-subj", f"/CN={domain_name}",
            "-days", "36500"
        ], check=True, capture_output=True)

        # 设置权限 - 确保Hysteria服务可以读取
        os.chmod(f"{target_dir}/{domain_name}.key", 0o644)
        os.chmod(f"{target_dir}/{domain_name}.crt", 0o644)
        os.chmod(target_dir, 0o755)  # 确保目录可访问

        # 清理临时文件
        try:
            os.remove(ec_param)
        except:
            pass

        print(f"\033[92m✓ 自签名证书生成成功\033[0m")
        print(f"   证书: {target_dir}/{domain_name}.crt")
        print(f"   私钥: {target_dir}/{domain_name}.key")
        return True

    except Exception as e:
        print(f"\033[91m✗ 证书生成失败: {e}\033[0m")
        return False

# ============================================
# 订阅链接生成
# ============================================

def generate_subscription_links(hy2_url):
    """使用新的订阅转换API生成配置"""
    print("\n正在生成客户端配置...")

    try:
        # URL编码
        encoded_url = urllib.parse.quote(hy2_url)

        # 预定义规则集（可根据需求调整）
        selected_rules = ["Ad Block", "AI Services", "Youtube", "Google",
                         "Private", "Location:CN", "Telegram", "Apple"]

        # 生成不同格式的配置
        configs = {
            "clash": f"{SUBSCRIPTION_API}/clash?config={encoded_url}",
            "singbox": f"{SUBSCRIPTION_API}/singbox?config={encoded_url}",
            "xray": f"{SUBSCRIPTION_API}/xray?config={encoded_url}"
        }

        config_dir = Path("/etc/hy2config")
        config_dir.mkdir(parents=True, exist_ok=True)

        success_count = 0
        for name, url in configs.items():
            try:
                response = requests.get(url, timeout=10)
                if response.status_code == 200:
                    config_file = config_dir / f"{name}.yaml"
                    config_file.write_text(response.text)
                    print(f"\033[92m✓ {name.capitalize()} 配置已生成\033[0m")
                    success_count += 1
                else:
                    print(f"\033[91m✗ {name.capitalize()} 配置生成失败 (HTTP {response.status_code})\033[0m")
            except Exception as e:
                print(f"\033[91m✗ {name.capitalize()} 配置生成失败: {e}\033[0m")

        if success_count > 0:
            print(f"\n\033[92m✓ 配置文件已保存到: {config_dir}\033[0m")
            return True
        else:
            print("\n\033[93m⚠ 未能生成任何配置文件\033[0m")
            return False

    except Exception as e:
        print(f"\033[91m✗ 订阅生成失败: {e}\033[0m")
        return False

# ============================================
# Hysteria2 配置
# ============================================

def hysteria2_config():
    """Hysteria2配置管理"""
    global hy2_domain, domain_name, insecure

    config_file = Path("/etc/hysteria/config.yaml")
    url_scheme_file = Path("/etc/hy2config/hy2_url_scheme.txt")

    while True:
        print("\n" + "=" * 60)
        print("Hysteria2 配置管理")
        print("=" * 60)
        print("1. 查看当前配置")
        print("2. 一键配置（推荐）")
        print("3. 手动编辑配置")
        print("4. 性能优化（可选）")
        print("0. 返回主菜单")
        print("=" * 60)

        choice = input("\n请选择 [0-4]: ").strip()

        if choice == "1":
            # 查看配置
            os.system("clear")
            print("=" * 60)
            print("当前配置")
            print("=" * 60)

            try:
                if config_file.exists():
                    print("\n【配置文件】\n")
                    print(config_file.read_text())
                else:
                    print("\033[91m✗ 未找到配置文件\033[0m")

                if url_scheme_file.exists():
                    print("\n【订阅链接】\n")
                    print(url_scheme_file.read_text())

                print("\n配置文件位置:")
                print(f"  - Clash: /etc/hy2config/clash.yaml")
                print(f"  - Sing-box: /etc/hy2config/singbox.yaml")
                print(f"  - Xray: /etc/hy2config/xray.yaml")

            except Exception as e:
                print(f"\033[91m✗ 读取配置失败: {e}\033[0m")

            input("\n按回车键继续...")

        elif choice == "2":
            # 一键配置
            try:
                # 基本配置
                print("\n" + "=" * 60)
                print("Hysteria2 配置向导")
                print("=" * 60)

                # 端口
                while True:
                    try:
                        port_input = input("\n监听端口 [默认 443]: ").strip()
                        hy2_port = int(port_input) if port_input else 443
                        if 1 <= hy2_port <= 65535:
                            break
                        print("\033[91m端口范围: 1-65535\033[0m")
                    except ValueError:
                        print("\033[91m请输入有效的端口号\033[0m")

                # 用户名和密码
                hy2_username = input("用户名 [默认 user]: ").strip() or "user"
                hy2_username = urllib.parse.quote(hy2_username)

                hy2_passwd = input("密码（建议使用强密码）: ").strip()
                if not hy2_passwd:
                    import random
                    import string
                    hy2_passwd = ''.join(random.choices(string.ascii_letters + string.digits, k=16))
                    print(f"\033[93m已生成随机密码: {hy2_passwd}\033[0m")

                # 伪装网站
                hy2_url = input("伪装网站 [默认 https://www.bing.com]: ").strip()
                hy2_url = hy2_url if hy2_url else "https://www.bing.com"

                # Brutal模式
                brutal_choice = input("\n开启 Brutal 模式？[y/N]: ").strip().lower()
                brutal_mode = "false" if brutal_choice == "y" else "true"

                # 混淆
                obfs_mode = ""
                obfs_scheme = ""
                obfs_choice = input("开启混淆模式？[y/N]: ").strip().lower()
                if obfs_choice == "y":
                    obfs_passwd = input("混淆密码: ").strip() or "salamander"
                    obfs_mode = f"obfs:\n  type: salamander\n  salamander:\n    password: {obfs_passwd}"
                    obfs_scheme = f"&obfs=salamander&obfs-password={urllib.parse.quote(obfs_passwd)}"

                # 协议嗅探
                sniff_mode = ""
                sniff_choice = input("开启协议嗅探？[y/N]: ").strip().lower()
                if sniff_choice == "y":
                    sniff_mode = "sniff:\n  enable: true\n  timeout: 2s\n  rewriteDomain: false\n  tcpPorts: 80,443,8000-9000\n  udpPorts: all"

                # 端口跳跃（简化版）
                jump_ports_hy2 = ""
                jump_choice = input("开启端口跳跃？[y/N]: ").strip().lower()
                if jump_choice == "y":
                    print("\n可用网络接口:")
                    os.system("ip -o link show | awk -F': ' '{print $2}'")
                    interface = input("网络接口名称 [默认 eth0]: ").strip() or "eth0"

                    try:
                        first_port = int(input("起始端口: "))
                        last_port = int(input("结束端口: "))

                        if 1 <= first_port <= last_port <= 65535:
                            # 设置iptables规则
                            subprocess.run(
                                f"iptables -t nat -A PREROUTING -i {interface} -p udp --dport {first_port}:{last_port} -j REDIRECT --to-ports {hy2_port}",
                                shell=True
                            )

                            # 保存恢复脚本
                            restore_script = Path("/etc/hy2config/jump_port_back.sh")
                            restore_script.write_text(f"#!/bin/bash\niptables -t nat -D PREROUTING -i {interface} -p udp --dport {first_port}:{last_port} -j REDIRECT --to-ports {hy2_port}\n")
                            restore_script.chmod(0o755)

                            jump_ports_hy2 = f"&mport={first_port}-{last_port}"
                            print("\033[92m✓ 端口跳跃已配置\033[0m")
                    except ValueError:
                        print("\033[91m✗ 端口配置无效，跳过端口跳跃\033[0m")

                # 证书配置
                print("\n" + "=" * 60)
                print("证书配置")
                print("=" * 60)
                print("1. 自动申请域名证书（需要域名）")
                print("2. 使用自签证书（无需域名）")
                print("3. 手动指定证书路径")

                cert_choice = input("\n请选择 [1-3]: ").strip()

                if cert_choice == "1":
                    # ACME证书
                    hy2_domain = input("域名: ").strip()
                    domain_name = hy2_domain
                    hy2_email = input("邮箱: ").strip()
                    insecure = "&insecure=0"

                    config_content = f"""listen: :{hy2_port}

acme:
  domains:
    - {hy2_domain}
  email: {hy2_email}

auth:
  type: password
  password: {hy2_passwd}

masquerade:
  type: proxy
  proxy:
    url: {hy2_url}
    rewriteHost: true

ignoreClientBandwidth: {brutal_mode}

{obfs_mode}
{sniff_mode}
"""

                elif cert_choice == "2":
                    # 自签证书
                    if not generate_self_signed_cert():
                        print("\033[91m✗ 证书生成失败\033[0m")
                        continue

                    # 获取IP
                    print("\n1. IPv4")
                    print("2. IPv6")
                    ip_choice = input("选择IP类型 [1/2]: ").strip()

                    if ip_choice == "2":
                        get_ipv6_address()
                    else:
                        get_ipv4_address()

                    insecure = "&insecure=1"

                    config_content = f"""listen: :{hy2_port}

tls:
  cert: /etc/ssl/private/{domain_name}.crt
  key: /etc/ssl/private/{domain_name}.key

auth:
  type: password
  password: {hy2_passwd}

masquerade:
  type: proxy
  proxy:
    url: {hy2_url}
    rewriteHost: true

ignoreClientBandwidth: {brutal_mode}

{obfs_mode}
{sniff_mode}
"""

                elif cert_choice == "3":
                    # 手动证书
                    cert_path = input("证书路径: ").strip()
                    key_path = input("私钥路径: ").strip()
                    hy2_domain = input("域名: ").strip()
                    domain_name = hy2_domain
                    insecure = "&insecure=0"

                    config_content = f"""listen: :{hy2_port}

tls:
  cert: {cert_path}
  key: {key_path}

auth:
  type: password
  password: {hy2_passwd}

masquerade:
  type: proxy
  proxy:
    url: {hy2_url}
    rewriteHost: true

ignoreClientBandwidth: {brutal_mode}

{obfs_mode}
{sniff_mode}
"""
                else:
                    print("\033[91m✗ 无效选项\033[0m")
                    continue

                # 写入配置
                config_file.parent.mkdir(parents=True, exist_ok=True)
                config_file.write_text(config_content)

                # 生成订阅链接
                os.system("clear")
                hy2_passwd_encoded = urllib.parse.quote(hy2_passwd)
                hy2_v2ray = f"hysteria2://{hy2_passwd_encoded}@{hy2_domain}:{hy2_port}?sni={domain_name}{obfs_scheme}{insecure}{jump_ports_hy2}#{hy2_username}"

                print("\n" + "=" * 60)
                print("配置完成")
                print("=" * 60)

                # 显示二维码
                print("\n【V2Ray 二维码】\n")
                subprocess.run(
                    f'echo "{hy2_v2ray}" | qrencode -s 1 -m 1 -t ANSI256 -o -',
                    shell=True
                )

                print(f"\n\n\033[92m【Hysteria2 链接】\033[0m\n{hy2_v2ray}\n")
                print("\033[93m支持客户端: v2rayN, NekoBox, v2rayNG, NekoRay\033[0m\n")

                # 保存链接
                url_scheme_file.parent.mkdir(parents=True, exist_ok=True)
                url_scheme_file.write_text(f"Hysteria2 订阅链接:\n{hy2_v2ray}\n")

                # 生成客户端配置
                generate_subscription_links(hy2_v2ray)

                # 启动服务
                print("\n正在启动服务...")
                subprocess.run("systemctl enable --now hysteria-server.service", shell=True)
                subprocess.run("systemctl restart hysteria-server.service", shell=True)

                print("\n\033[92m✓ Hysteria2 配置完成并已启动\033[0m")
                input("\n按回车键继续...")

            except Exception as e:
                print(f"\033[91m✗ 配置失败: {e}\033[0m")
                input("\n按回车键继续...")

        elif choice == "3":
            # 手动编辑
            print("\n\033[93m提示: 使用 nano 编辑器，Ctrl+X 保存退出\033[0m")
            subprocess.run("nano /etc/hysteria/config.yaml", shell=True)
            subprocess.run("systemctl restart hysteria-server.service", shell=True)
            print("\n\033[92m✓ 服务已重启\033[0m")

        elif choice == "4":
            # 性能优化
            print("\n正在下载BBR优化脚本...")
            subprocess.run(
                "wget -O /tmp/tcpx.sh 'https://github.com/ylx2016/Linux-NetSpeed/raw/master/tcpx.sh' && chmod +x /tmp/tcpx.sh && /tmp/tcpx.sh",
                shell=True
            )

        elif choice == "0":
            break
        else:
            print("\033[91m无效选项\033[0m")

# ============================================
# 主程序
# ============================================

def show_banner():
    """显示欢迎信息"""
    os.system("clear")
    print("\033[94m" + "=" * 60)
    print("""
    ╦ ╦╦ ╦╔═╗╔╦╗╔═╗╦═╗╦╔═╗  ╔═╗  ╦╔╗╔╔═╗╔╦╗╔═╗╦  ╦
    ╠═╣╚╦╝╚═╗ ║ ║╣ ╠╦╝║╠═╣  ╔═╝  ║║║║╚═╗ ║ ╠═╣║  ║
    ╩ ╩ ╩ ╚═╝ ╩ ╚═╝╩╚═╩╩ ╩  ╚═╝  ╩╝╚╝╚═╝ ╩ ╩ ╩╩═╝╩═╝
    """)
    print(f"    版本: {VERSION}  |  一键安装、配置、管理")
    print("=" * 60 + "\033[0m")
    print()

def main():
    """主函数"""
    # 检查root权限
    if os.geteuid() != 0:
        print("\033[91m✗ 请使用 root 权限运行此脚本\033[0m")
        print("使用: sudo python3", sys.argv[0])
        sys.exit(1)

    # 安装依赖
    if not install_system_dependencies():
        print("\033[91m✗ 依赖安装失败\033[0m")
        sys.exit(1)

    # 设置环境
    if not setup_environment():
        sys.exit(1)

    # 主循环
    while True:
        show_banner()
        print("\033[93m提示: 输入 'hy2' 快捷启动\033[0m\n")
        print("1. 安装/更新 Hysteria2")
        print("2. 卸载 Hysteria2")
        print("3. 配置管理")
        print("4. 服务管理")
        print("0. 退出")
        print("\n" + "=" * 60)

        choice = input("\n请选择 [0-4]: ").strip()

        if choice == "1":
            hysteria2_install()
        elif choice == "2":
            hysteria2_uninstall()
        elif choice == "3":
            hysteria2_config()
        elif choice == "4":
            server_manage()
        elif choice == "0":
            print("\n再见！")
            sys.exit(0)
        else:
            print("\033[91m无效选项\033[0m")
            time.sleep(1)

if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\n\n\033[93m✗ 用户中断\033[0m")
        sys.exit(0)
    except Exception as e:
        print(f"\n\033[91m✗ 发生错误: {e}\033[0m")
        sys.exit(1)
