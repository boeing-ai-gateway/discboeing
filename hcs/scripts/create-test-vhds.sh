#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/create-test-vhds.sh [output-directory]

Creates two fixed VHD files for Windows HCS smoke testing:
  hcs-test-root.vhd  read-only ext4 root disk with a static /init
  hcs-test-data.vhd  read/write ext4 data disk

Requirements on the build machine:
  gcc, mkfs.ext4, debugfs, python3, truncate

Optional user-mode networking test support:
  GVFORWARDER=/path/to/gvforwarder scripts/create-test-vhds.sh [output-directory]

When GVFORWARDER is set, the root disk also includes:
  /sbin/gvforwarder  gvisor-tap-vsock guest forwarder
  /sbin/udhcpc       tiny static network helper used by gvforwarder

The helper reads static user networking settings from the kernel command line:
  discobot=ip=192.168.127.2,netmask=255.255.255.0,gateway=192.168.127.1,dns=192.168.127.1
It configures tap0, verifies gvproxy DNS for host.containers.internal, and
writes /data/usernet-ok.txt.

The root disk intentionally has an ext4 filesystem directly on /dev/sda
(no partition table), so launch it with:
  --root-device /dev/sda --no-initrd --append-kernel-cmdline "init=/init"
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

out_dir="${1:-artifacts/test-disks}"
gvforwarder="${GVFORWARDER:-}"
gvproxy_vsock_port="${GVPROXY_VSOCK_PORT:-1024}"
tun_module="${TUN_MODULE:-}"

if [[ -n "$gvforwarder" && -z "$tun_module" ]]; then
  tun_module="$(find /lib/modules -name tun.ko -print -quit 2>/dev/null || true)"
fi

if [[ -n "$gvforwarder" && ! -f "$gvforwarder" ]]; then
  echo "GVFORWARDER was set but the file was not found: $gvforwarder" >&2
  exit 1
fi

if [[ -n "$gvforwarder" && ! -f "$tun_module" ]]; then
  echo "GVFORWARDER requires a tun.ko module. Set TUN_MODULE=/path/to/tun.ko." >&2
  exit 1
fi

if [[ -n "$gvforwarder" && -z "${ROOT_SIZE_MB:-}" ]]; then
  root_size_mb=160
else
  root_size_mb="${ROOT_SIZE_MB:-64}"
fi
data_size_mb="${DATA_SIZE_MB:-32}"

for tool in gcc mkfs.ext4 debugfs python3 truncate; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "Missing required tool: $tool" >&2
    exit 1
  fi
done

mkdir -p "$out_dir"
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

cat >"$work_dir/test-init.c" <<'EOF_C'
#define _GNU_SOURCE
#include <errno.h>
#include <fcntl.h>
#include <stdarg.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mount.h>
#include <sys/reboot.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/sysmacros.h>
#include <sys/types.h>
#include <time.h>
#include <unistd.h>

#ifndef GVPROXY_VSOCK_PORT
#define GVPROXY_VSOCK_PORT 1024
#endif

static void write_all(int fd, const char* text)
{
    size_t remaining = strlen(text);
    while (remaining > 0) {
        ssize_t written = write(fd, text, remaining);
        if (written <= 0) {
            return;
        }

        text += written;
        remaining -= (size_t)written;
    }
}

static void log_msg(const char* format, ...)
{
    char buffer[1024];
    va_list args;
    va_start(args, format);
    int count = vsnprintf(buffer, sizeof(buffer), format, args);
    va_end(args);
    if (count < 0) {
        return;
    }

    if ((size_t)count >= sizeof(buffer)) {
        count = (int)sizeof(buffer) - 1;
        buffer[count] = '\0';
    }

    int fd = open("/dev/console", O_WRONLY | O_NOCTTY | O_CLOEXEC);
    if (fd >= 0) {
        write_all(fd, buffer);
        close(fd);
    }

    fd = open("/dev/kmsg", O_WRONLY | O_CLOEXEC);
    if (fd >= 0) {
        write_all(fd, "hcs-test-init: ");
        write_all(fd, buffer);
        close(fd);
    }
}

static bool mount_if_available(const char* source, const char* target, const char* type, unsigned long flags, const char* data)
{
    if (mount(source, target, type, flags, data) == 0 || errno == EBUSY) {
        return true;
    }

    log_msg("mount %s on %s as %s failed: %s\n", source, target, type, strerror(errno));
    return false;
}

static void load_tun_module_if_present(void)
{
    int fd = open("/lib/modules/tun.ko", O_RDONLY | O_CLOEXEC);
    if (fd < 0) {
        return;
    }

    long rc = syscall(SYS_finit_module, fd, "", 0);
    if (rc != 0 && errno != EEXIST) {
        log_msg("loading /lib/modules/tun.ko failed: %s\n", strerror(errno));
    } else {
        log_msg("loaded /lib/modules/tun.ko\n");
    }

    close(fd);
}

static void ensure_dev_net_tun(void)
{
    load_tun_module_if_present();

    if (mkdir("/dev/net", 0755) != 0 && errno != EEXIST) {
        log_msg("mkdir /dev/net failed: %s\n", strerror(errno));
        return;
    }

    if (mknod("/dev/net/tun", S_IFCHR | 0666, makedev(10, 200)) != 0 && errno != EEXIST) {
        log_msg("mknod /dev/net/tun failed: %s\n", strerror(errno));
    }
}

static void start_gvforwarder_if_present(void)
{
    if (access("/sbin/gvforwarder", X_OK) != 0) {
        return;
    }

    ensure_dev_net_tun();
    setenv("PATH", "/sbin:/bin:/usr/sbin:/usr/bin", 1);

    char url[64];
    snprintf(url, sizeof(url), "vsock://2:%d/connect", GVPROXY_VSOCK_PORT);

    pid_t pid = fork();
    if (pid < 0) {
        log_msg("fork gvforwarder failed: %s\n", strerror(errno));
        return;
    }

    if (pid == 0) {
        int console = open("/dev/console", O_RDWR | O_NOCTTY);
        if (console >= 0) {
            dup2(console, STDIN_FILENO);
            dup2(console, STDOUT_FILENO);
            dup2(console, STDERR_FILENO);
            if (console > STDERR_FILENO) {
                close(console);
            }
        }

        execl("/sbin/gvforwarder", "gvforwarder", "-debug", "-url", url, "-stop-if-exist", "", (char*)NULL);
        _exit(127);
    }

    log_msg("started gvforwarder pid=%d url=%s\n", (int)pid, url);
}

static void wait_for_usernet_marker(void)
{
    if (access("/sbin/gvforwarder", X_OK) != 0) {
        return;
    }

    for (int attempt = 0; attempt < 90; attempt++) {
        if (access("/data/usernet-ok.txt", F_OK) == 0) {
            log_msg("usernet marker appeared\n");
            return;
        }

        sleep(1);
    }

    int fd = open("/data/usernet-timeout.txt", O_CREAT | O_TRUNC | O_WRONLY | O_CLOEXEC, 0644);
    if (fd >= 0) {
        dprintf(fd, "gvforwarder usernet marker did not appear within timeout\n");
        close(fd);
        sync();
    }

    log_msg("gvforwarder usernet marker did not appear within timeout\n");
}

int main(void)
{
    log_msg("hcs-test-init started\n");

    mount_if_available("proc", "/proc", "proc", 0, "");
    mount_if_available("sysfs", "/sys", "sysfs", 0, "");
    mount_if_available("devtmpfs", "/dev", "devtmpfs", 0, "mode=0755");
    mount_if_available("tmpfs", "/etc", "tmpfs", 0, "mode=0755");

    int mounted = 0;
    for (int attempt = 0; attempt < 60; attempt++) {
        if (mount("/dev/sdb", "/data", "ext4", MS_RELATIME, "") == 0) {
            mounted = 1;
            break;
        }

        if (errno != ENOENT && errno != ENXIO && errno != ENODEV) {
            log_msg("mount /dev/sdb on /data failed on attempt %d: %s\n", attempt + 1, strerror(errno));
        }

        sleep(1);
    }

    if (!mounted) {
        log_msg("data disk /dev/sdb did not mount; smoke test failed\n");
        while (1) {
            sleep(3600);
        }
    }

    int fd = open("/data/boot-ok.txt", O_CREAT | O_TRUNC | O_WRONLY | O_CLOEXEC, 0644);
    if (fd >= 0) {
        time_t now = time(NULL);
        dprintf(fd, "hcs-test-init booted successfully\n");
        dprintf(fd, "unix-time=%lld\n", (long long)now);
        dprintf(fd, "root=/dev/sda read-only, data=/dev/sdb read-write\n");
        close(fd);
        sync();
        log_msg("wrote /data/boot-ok.txt\n");
    } else {
        log_msg("failed to write /data/boot-ok.txt: %s\n", strerror(errno));
    }

    start_gvforwarder_if_present();
    wait_for_usernet_marker();

    while (1) {
        sleep(3600);
    }
}
EOF_C

gcc -Os -static -s -DGVPROXY_VSOCK_PORT="$gvproxy_vsock_port" -o "$work_dir/init" "$work_dir/test-init.c"

if [[ -n "$gvforwarder" ]]; then
  cat >"$work_dir/mini-udhcpc.c" <<'EOF_C'
#define _GNU_SOURCE
#include <arpa/inet.h>
#include <errno.h>
#include <fcntl.h>
#include <linux/if.h>
#include <net/route.h>
#include <netinet/in.h>
#include <stdarg.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/time.h>
#include <sys/types.h>
#include <time.h>
#include <unistd.h>

#define DHCP_CLIENT_PORT 68
#define DHCP_SERVER_PORT 67
#define DHCP_MAGIC 0x63825363U
#define DHCP_DISCOVER 1
#define DHCP_OFFER 2
#define DHCP_REQUEST 3
#define DHCP_ACK 5

struct dhcp_packet {
    uint8_t op, htype, hlen, hops;
    uint32_t xid;
    uint16_t secs, flags;
    uint32_t ciaddr, yiaddr, siaddr, giaddr;
    uint8_t chaddr[16];
    uint8_t sname[64];
    uint8_t file[128];
    uint32_t magic;
    uint8_t options[312];
} __attribute__((packed));

static void log_msg(const char* format, ...)
{
    va_list args;
    va_start(args, format);
    vfprintf(stderr, format, args);
    va_end(args);
}

static const char* ip_text(uint32_t addr, char* buffer, size_t size)
{
    struct in_addr in = { .s_addr = addr };
    const char* text = inet_ntop(AF_INET, &in, buffer, (socklen_t)size);
    return text ? text : "0.0.0.0";
}

static int get_hwaddr(const char* iface, uint8_t mac[6])
{
    int fd = socket(AF_INET, SOCK_DGRAM | SOCK_CLOEXEC, 0);
    if (fd < 0) return -1;
    struct ifreq ifr;
    memset(&ifr, 0, sizeof(ifr));
    strncpy(ifr.ifr_name, iface, IFNAMSIZ - 1);
    int rc = ioctl(fd, SIOCGIFHWADDR, &ifr);
    close(fd);
    if (rc != 0) return -1;
    memcpy(mac, ifr.ifr_hwaddr.sa_data, 6);
    return 0;
}

static uint8_t* add_option(uint8_t* opt, uint8_t code, uint8_t len, const void* data)
{
    *opt++ = code;
    *opt++ = len;
    memcpy(opt, data, len);
    return opt + len;
}

static size_t build_packet(struct dhcp_packet* pkt, uint8_t type, uint32_t xid, const uint8_t mac[6], uint32_t requested_ip, uint32_t server_id)
{
    memset(pkt, 0, sizeof(*pkt));
    pkt->op = 1;
    pkt->htype = 1;
    pkt->hlen = 6;
    pkt->xid = htonl(xid);
    pkt->flags = htons(0x8000);
    memcpy(pkt->chaddr, mac, 6);
    pkt->magic = htonl(DHCP_MAGIC);

    uint8_t* opt = pkt->options;
    opt = add_option(opt, 53, 1, &type);
    if (requested_ip != 0) opt = add_option(opt, 50, 4, &requested_ip);
    if (server_id != 0) opt = add_option(opt, 54, 4, &server_id);
    uint8_t params[] = { 1, 3, 6, 15, 28, 51, 58, 59 };
    opt = add_option(opt, 55, sizeof(params), params);
    *opt++ = 255;
    return (size_t)((uint8_t*)opt - (uint8_t*)pkt);
}

static bool get_option(const struct dhcp_packet* pkt, uint8_t code, const uint8_t** value, uint8_t* length)
{
    const uint8_t* opt = pkt->options;
    const uint8_t* end = pkt->options + sizeof(pkt->options);
    while (opt < end && *opt != 255) {
        if (*opt == 0) { opt++; continue; }
        if (opt + 2 > end) break;
        uint8_t current = opt[0];
        uint8_t len = opt[1];
        opt += 2;
        if (opt + len > end) break;
        if (current == code) {
            *value = opt;
            *length = len;
            return true;
        }
        opt += len;
    }
    return false;
}

static int wait_for_dhcp(int fd, uint32_t xid, uint8_t expected_type, struct dhcp_packet* out)
{
    for (;;) {
        struct dhcp_packet pkt;
        ssize_t n = recv(fd, &pkt, sizeof(pkt), 0);
        if (n < 0) return -1;
        if ((size_t)n < offsetof(struct dhcp_packet, options)) continue;
        if (ntohl(pkt.magic) != DHCP_MAGIC || ntohl(pkt.xid) != xid) continue;
        const uint8_t* value;
        uint8_t length;
        if (!get_option(&pkt, 53, &value, &length) || length != 1) continue;
        if (value[0] != expected_type) continue;
        memcpy(out, &pkt, sizeof(pkt));
        return 0;
    }
}

static int set_addr(const char* iface, unsigned long request, uint32_t addr)
{
    int fd = socket(AF_INET, SOCK_DGRAM | SOCK_CLOEXEC, 0);
    if (fd < 0) return -1;
    struct ifreq ifr;
    memset(&ifr, 0, sizeof(ifr));
    strncpy(ifr.ifr_name, iface, IFNAMSIZ - 1);
    struct sockaddr_in* sin = (struct sockaddr_in*)&ifr.ifr_addr;
    sin->sin_family = AF_INET;
    sin->sin_addr.s_addr = addr;
    int rc = ioctl(fd, request, &ifr);
    close(fd);
    return rc;
}

static int set_mtu(const char* iface, int mtu)
{
    int fd = socket(AF_INET, SOCK_DGRAM | SOCK_CLOEXEC, 0);
    if (fd < 0) return -1;
    struct ifreq ifr;
    memset(&ifr, 0, sizeof(ifr));
    strncpy(ifr.ifr_name, iface, IFNAMSIZ - 1);
    ifr.ifr_mtu = mtu;
    int rc = ioctl(fd, SIOCSIFMTU, &ifr);
    close(fd);
    return rc;
}

static int add_default_route(const char* iface, uint32_t gateway)
{
    int fd = socket(AF_INET, SOCK_DGRAM | SOCK_CLOEXEC, 0);
    if (fd < 0) return -1;
    struct rtentry route;
    memset(&route, 0, sizeof(route));
    struct sockaddr_in* dst = (struct sockaddr_in*)&route.rt_dst;
    struct sockaddr_in* gw = (struct sockaddr_in*)&route.rt_gateway;
    struct sockaddr_in* mask = (struct sockaddr_in*)&route.rt_genmask;
    dst->sin_family = AF_INET;
    gw->sin_family = AF_INET;
    mask->sin_family = AF_INET;
    gw->sin_addr.s_addr = gateway;
    route.rt_flags = RTF_UP | RTF_GATEWAY;
    route.rt_dev = (char*)iface;
    int rc = ioctl(fd, SIOCADDRT, &route);
    if (rc != 0 && errno == EEXIST) rc = 0;
    close(fd);
    return rc;
}

static int dns_query(uint32_t dns_server, uint32_t* answer)
{
    uint8_t query[512];
    memset(query, 0, sizeof(query));
    uint16_t id = (uint16_t)(time(NULL) ^ getpid());
    query[0] = (uint8_t)(id >> 8);
    query[1] = (uint8_t)id;
    query[2] = 1; // recursion desired
    query[5] = 1; // qdcount
    size_t pos = 12;
    const char* labels[] = { "host", "containers", "internal" };
    for (size_t i = 0; i < 3; i++) {
        size_t len = strlen(labels[i]);
        query[pos++] = (uint8_t)len;
        memcpy(query + pos, labels[i], len);
        pos += len;
    }
    query[pos++] = 0;
    query[pos++] = 0; query[pos++] = 1; // A
    query[pos++] = 0; query[pos++] = 1; // IN

    int fd = socket(AF_INET, SOCK_DGRAM | SOCK_CLOEXEC, 0);
    if (fd < 0) return -1;
    struct timeval timeout = { .tv_sec = 5, .tv_usec = 0 };
    setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &timeout, sizeof(timeout));
    struct sockaddr_in server;
    memset(&server, 0, sizeof(server));
    server.sin_family = AF_INET;
    server.sin_port = htons(53);
    server.sin_addr.s_addr = dns_server;
    if (sendto(fd, query, pos, 0, (struct sockaddr*)&server, sizeof(server)) < 0) { close(fd); return -1; }
    uint8_t response[512];
    ssize_t n = recv(fd, response, sizeof(response), 0);
    close(fd);
    if (n < 12 || response[0] != query[0] || response[1] != query[1]) return -1;
    int ancount = (response[6] << 8) | response[7];
    pos = 12;
    while (pos < (size_t)n && response[pos] != 0) pos += response[pos] + 1;
    pos += 5; // zero label + qtype/qclass
    for (int i = 0; i < ancount && pos + 12 <= (size_t)n; i++) {
        if ((response[pos] & 0xc0) == 0xc0) pos += 2;
        else { while (pos < (size_t)n && response[pos] != 0) pos += response[pos] + 1; pos++; }
        if (pos + 10 > (size_t)n) return -1;
        uint16_t type = (response[pos] << 8) | response[pos + 1];
        uint16_t rdlen = (response[pos + 8] << 8) | response[pos + 9];
        pos += 10;
        if (type == 1 && rdlen == 4 && pos + 4 <= (size_t)n) {
            memcpy(answer, response + pos, 4);
            return 0;
        }
        pos += rdlen;
    }
    return -1;
}

static void write_marker(const char* mode, const char* iface, uint32_t ip, uint32_t mask, uint32_t router, uint32_t dns, int dns_ok, uint32_t dns_answer)
{
    int fd = open("/data/usernet-ok.txt", O_CREAT | O_TRUNC | O_WRONLY | O_CLOEXEC, 0644);
    if (fd < 0) return;
    char ipb[INET_ADDRSTRLEN], maskb[INET_ADDRSTRLEN], routerb[INET_ADDRSTRLEN], dnsb[INET_ADDRSTRLEN], ansb[INET_ADDRSTRLEN];
    dprintf(fd, "gvforwarder usernet configured\n");
    dprintf(fd, "mode=%s\n", mode);
    dprintf(fd, "interface=%s\n", iface);
    dprintf(fd, "ip=%s\n", ip_text(ip, ipb, sizeof(ipb)));
    dprintf(fd, "netmask=%s\n", ip_text(mask, maskb, sizeof(maskb)));
    dprintf(fd, "router=%s\n", ip_text(router, routerb, sizeof(routerb)));
    dprintf(fd, "dns=%s\n", ip_text(dns, dnsb, sizeof(dnsb)));
    dprintf(fd, "dns_query_host_containers_internal=%s\n", dns_ok == 0 ? "ok" : "failed");
    if (dns_ok == 0) dprintf(fd, "dns_answer=%s\n", ip_text(dns_answer, ansb, sizeof(ansb)));
    close(fd);
    sync();
}

static int configure_static(const char* mode, const char* iface, uint32_t ip, uint32_t mask, uint32_t router, uint32_t dns)
{
    set_mtu(iface, 1500);
    if (set_addr(iface, SIOCSIFADDR, ip) != 0) log_msg("mini-udhcpc: set static ip failed: %s\n", strerror(errno));
    if (set_addr(iface, SIOCSIFNETMASK, mask) != 0) log_msg("mini-udhcpc: set static netmask failed: %s\n", strerror(errno));
    if (router != 0 && add_default_route(iface, router) != 0) log_msg("mini-udhcpc: add static route failed: %s\n", strerror(errno));

    if (dns != 0) {
        FILE* resolv = fopen("/etc/resolv.conf", "w");
        if (resolv) {
            char dnsb[INET_ADDRSTRLEN];
            fprintf(resolv, "nameserver %s\n", ip_text(dns, dnsb, sizeof(dnsb)));
            fclose(resolv);
        }
    }

    uint32_t dns_answer = 0;
    int dns_ok = dns != 0 ? dns_query(dns, &dns_answer) : -1;
    write_marker(mode, iface, ip, mask, router, dns, dns_ok, dns_answer);
    return 0;
}

static bool parse_discobot_config(uint32_t* ip, uint32_t* mask, uint32_t* router, uint32_t* dns)
{
    char buffer[4096];
    int fd = open("/proc/cmdline", O_RDONLY | O_CLOEXEC);
    if (fd < 0) return false;
    ssize_t n = read(fd, buffer, sizeof(buffer) - 1);
    close(fd);
    if (n <= 0) return false;
    buffer[n] = '\0';

    char* start = strstr(buffer, "discobot=");
    if (!start) return false;
    start += strlen("discobot=");
    char* end = start;
    while (*end && *end != ' ' && *end != '\n' && *end != '\r' && *end != '\t') end++;
    *end = '\0';

    bool have_ip = false;
    bool have_mask = false;
    bool have_dns = false;
    bool have_router = false;
    for (char* part = strtok(start, ","); part; part = strtok(NULL, ",")) {
        char* value = strchr(part, '=');
        if (!value) continue;
        *value++ = '\0';
        if (strcmp(part, "ip") == 0) {
            *ip = inet_addr(value);
            have_ip = true;
        } else if (strcmp(part, "netmask") == 0) {
            *mask = inet_addr(value);
            have_mask = true;
        } else if (strcmp(part, "gateway") == 0 || strcmp(part, "gw") == 0) {
            *router = inet_addr(value);
            have_router = true;
        } else if (strcmp(part, "dns") == 0) {
            *dns = inet_addr(value);
            have_dns = true;
        }
    }

    if (!have_router && have_dns) {
        *router = *dns;
        have_router = true;
    }

    return have_ip && have_mask && have_router && have_dns;
}

int main(int argc, char** argv)
{
    const char* iface = "tap0";
    for (int i = 1; i + 1 < argc; i++) {
        if (strcmp(argv[i], "-i") == 0) iface = argv[i + 1];
    }

    uint32_t static_ip = 0;
    uint32_t static_mask = 0;
    uint32_t static_router = 0;
    uint32_t static_dns = 0;
    if (parse_discobot_config(&static_ip, &static_mask, &static_router, &static_dns)) {
        log_msg("mini-udhcpc: configuring static discobot usernet\n");
        return configure_static("kernel-cmdline-static", iface, static_ip, static_mask, static_router, static_dns);
    }

    uint8_t mac[6];
    if (get_hwaddr(iface, mac) != 0) {
        log_msg("mini-udhcpc: failed to read mac for %s: %s\n", iface, strerror(errno));
        return 1;
    }

    int fd = socket(AF_INET, SOCK_DGRAM | SOCK_CLOEXEC, IPPROTO_UDP);
    if (fd < 0) return 1;
    int one = 1;
    setsockopt(fd, SOL_SOCKET, SO_BROADCAST, &one, sizeof(one));
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &one, sizeof(one));
    setsockopt(fd, SOL_SOCKET, SO_BINDTODEVICE, iface, strlen(iface));
    struct timeval timeout = { .tv_sec = 10, .tv_usec = 0 };
    setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &timeout, sizeof(timeout));

    struct sockaddr_in local;
    memset(&local, 0, sizeof(local));
    local.sin_family = AF_INET;
    local.sin_port = htons(DHCP_CLIENT_PORT);
    local.sin_addr.s_addr = INADDR_ANY;
    if (bind(fd, (struct sockaddr*)&local, sizeof(local)) != 0) {
        log_msg("mini-udhcpc: bind failed: %s\n", strerror(errno));
        close(fd);
        return 1;
    }

    struct sockaddr_in dest;
    memset(&dest, 0, sizeof(dest));
    dest.sin_family = AF_INET;
    dest.sin_port = htons(DHCP_SERVER_PORT);
    dest.sin_addr.s_addr = INADDR_BROADCAST;

    uint32_t xid = (uint32_t)(time(NULL) ^ getpid());
    struct dhcp_packet pkt;
    size_t len = build_packet(&pkt, DHCP_DISCOVER, xid, mac, 0, 0);
    if (sendto(fd, &pkt, len, 0, (struct sockaddr*)&dest, sizeof(dest)) < 0) {
        log_msg("mini-udhcpc: discover send failed: %s\n", strerror(errno));
        close(fd);
        return 1;
    }

    struct dhcp_packet offer;
    if (wait_for_dhcp(fd, xid, DHCP_OFFER, &offer) != 0) {
        log_msg("mini-udhcpc: timed out waiting for offer; using gvproxy static defaults\n");
        close(fd);
        uint32_t ip = inet_addr("192.168.127.2");
        uint32_t mask = inet_addr("255.255.255.0");
        uint32_t router = inet_addr("192.168.127.1");
        uint32_t dns = inet_addr("192.168.127.1");
        set_mtu(iface, 1500);
        set_addr(iface, SIOCSIFADDR, ip);
        set_addr(iface, SIOCSIFNETMASK, mask);
        add_default_route(iface, router);
        FILE* resolv = fopen("/etc/resolv.conf", "w");
        if (resolv) {
            fprintf(resolv, "nameserver 192.168.127.1\n");
            fclose(resolv);
        }
        uint32_t dns_answer = 0;
        int dns_ok = dns_query(dns, &dns_answer);
        write_marker("static-fallback-after-dhcp-offer-timeout", iface, ip, mask, router, dns, dns_ok, dns_answer);
        return 0;
    }

    const uint8_t* value;
    uint8_t opt_len;
    uint32_t server_id = offer.siaddr;
    if (get_option(&offer, 54, &value, &opt_len) && opt_len == 4) memcpy(&server_id, value, 4);
    uint32_t requested_ip = offer.yiaddr;
    len = build_packet(&pkt, DHCP_REQUEST, xid, mac, requested_ip, server_id);
    if (sendto(fd, &pkt, len, 0, (struct sockaddr*)&dest, sizeof(dest)) < 0) {
        log_msg("mini-udhcpc: request send failed: %s\n", strerror(errno));
        close(fd);
        return 1;
    }

    struct dhcp_packet ack;
    if (wait_for_dhcp(fd, xid, DHCP_ACK, &ack) != 0) {
        log_msg("mini-udhcpc: timed out waiting for ack\n");
        close(fd);
        return 1;
    }
    close(fd);

    uint32_t ip = ack.yiaddr;
    uint32_t mask = inet_addr("255.255.255.0");
    uint32_t router = 0;
    uint32_t dns = 0;
    if (get_option(&ack, 1, &value, &opt_len) && opt_len >= 4) memcpy(&mask, value, 4);
    if (get_option(&ack, 3, &value, &opt_len) && opt_len >= 4) memcpy(&router, value, 4);
    if (get_option(&ack, 6, &value, &opt_len) && opt_len >= 4) memcpy(&dns, value, 4);

    set_mtu(iface, 1500);
    if (set_addr(iface, SIOCSIFADDR, ip) != 0) log_msg("mini-udhcpc: set ip failed: %s\n", strerror(errno));
    if (set_addr(iface, SIOCSIFNETMASK, mask) != 0) log_msg("mini-udhcpc: set netmask failed: %s\n", strerror(errno));
    if (router != 0 && add_default_route(iface, router) != 0) log_msg("mini-udhcpc: add route failed: %s\n", strerror(errno));

    if (dns != 0) {
        FILE* resolv = fopen("/etc/resolv.conf", "w");
        if (resolv) {
            char dnsb[INET_ADDRSTRLEN];
            fprintf(resolv, "nameserver %s\n", ip_text(dns, dnsb, sizeof(dnsb)));
            fclose(resolv);
        }
    }

    uint32_t dns_answer = 0;
    int dns_ok = dns != 0 ? dns_query(dns, &dns_answer) : -1;
    write_marker("dhcp", iface, ip, mask, router, dns, dns_ok, dns_answer);
    return 0;
}
EOF_C
  gcc -Os -static -s -o "$work_dir/udhcpc" "$work_dir/mini-udhcpc.c"
fi

create_ext4() {
  local path="$1"
  local size_mb="$2"
  local label="$3"
  truncate -s "${size_mb}M" "$path"
  mkfs.ext4 -q -F -L "$label" "$path"
}

root_raw="$work_dir/root.raw"
data_raw="$work_dir/data.raw"
create_ext4 "$root_raw" "$root_size_mb" HCSROOT
create_ext4 "$data_raw" "$data_size_mb" HCSDATA

cat >"$work_dir/debugfs.cmd" <<EOF_DEBUGFS
mkdir /proc
mkdir /sys
mkdir /dev
mkdir /etc
mkdir /data
mkdir /sbin
mkdir /lib
mkdir /lib/modules
write $work_dir/init /init
sif /init mode 0100755
EOF_DEBUGFS

if [[ -n "$gvforwarder" ]]; then
  cat >>"$work_dir/debugfs.cmd" <<EOF_DEBUGFS
write $gvforwarder /sbin/gvforwarder
sif /sbin/gvforwarder mode 0100755
write $work_dir/udhcpc /sbin/udhcpc
sif /sbin/udhcpc mode 0100755
write $tun_module /lib/modules/tun.ko
EOF_DEBUGFS
fi

debugfs -w -f "$work_dir/debugfs.cmd" "$root_raw" >/dev/null 2>&1

python3 - "$root_raw" "$out_dir/hcs-test-root.vhd" "$data_raw" "$out_dir/hcs-test-data.vhd" <<'PY'
import os
import struct
import sys
import time
import uuid

SECTOR_SIZE = 512
VHD_EPOCH_OFFSET = 946684800  # seconds from Unix epoch to 2000-01-01 UTC


def chs_geometry(size_bytes: int) -> tuple[int, int, int]:
    total_sectors = size_bytes // SECTOR_SIZE
    max_sectors = 65535 * 16 * 255
    if total_sectors > max_sectors:
        total_sectors = max_sectors

    if total_sectors >= 65535 * 16 * 63:
        sectors_per_track = 255
        heads = 16
        cylinders = total_sectors // (heads * sectors_per_track)
    else:
        sectors_per_track = 17
        cylinders = total_sectors // sectors_per_track
        heads = (cylinders + 1023) // 1024
        if heads < 4:
            heads = 4
        if cylinders >= heads * 1024 or heads > 16:
            sectors_per_track = 31
            heads = 16
            cylinders = total_sectors // (heads * sectors_per_track)
        if cylinders >= 1024:
            sectors_per_track = 63
            heads = 16
            cylinders = total_sectors // (heads * sectors_per_track)

    return int(cylinders), int(heads), int(sectors_per_track)


def fixed_vhd_footer(size_bytes: int) -> bytes:
    if size_bytes % SECTOR_SIZE != 0:
        raise ValueError("VHD virtual size must be sector-aligned")

    cylinders, heads, sectors = chs_geometry(size_bytes)
    footer = bytearray(512)
    footer[0:8] = b"conectix"
    struct.pack_into(">I", footer, 8, 0x00000002)
    struct.pack_into(">I", footer, 12, 0x00010000)
    struct.pack_into(">Q", footer, 16, 0xFFFFFFFFFFFFFFFF)
    struct.pack_into(">I", footer, 24, int(time.time() - VHD_EPOCH_OFFSET))
    footer[28:32] = b"dscb"
    struct.pack_into(">I", footer, 32, 0x00010000)
    footer[36:40] = b"Wi2k"
    struct.pack_into(">Q", footer, 40, size_bytes)
    struct.pack_into(">Q", footer, 48, size_bytes)
    struct.pack_into(">HBB", footer, 56, cylinders, heads, sectors)
    struct.pack_into(">I", footer, 60, 2)
    footer[68:84] = uuid.uuid4().bytes
    footer[84] = 0

    checksum = (~sum(footer) & 0xFFFFFFFF)
    struct.pack_into(">I", footer, 64, checksum)
    return bytes(footer)


def convert(raw_path: str, vhd_path: str) -> None:
    size = os.path.getsize(raw_path)
    with open(raw_path, "rb") as src, open(vhd_path, "wb") as dst:
        while True:
            block = src.read(1024 * 1024)
            if not block:
                break
            dst.write(block)
        dst.write(fixed_vhd_footer(size))


args = sys.argv[1:]
if len(args) % 2:
    raise SystemExit("expected raw/vhd path pairs")
for raw, vhd in zip(args[0::2], args[1::2]):
    convert(raw, vhd)
    print(f"created {vhd} ({os.path.getsize(vhd)} bytes)")
PY

cat <<EOF_DONE

Created test disks:
  $out_dir/hcs-test-root.vhd
  $out_dir/hcs-test-data.vhd
EOF_DONE

if [[ -n "$gvforwarder" ]]; then
  cat <<EOF_DONE

Embedded gvforwarder user-mode networking test support:
  gvforwarder: $gvforwarder
  tun module: $tun_module
  gvproxy VSOCK port: $gvproxy_vsock_port

Windows user-network smoke-test command:
  dotnet run -- --root C:\\vm\\hcs-test-root.vhd --data C:\\vm\\hcs-test-data.vhd --root-device /dev/sda --no-initrd --append-kernel-cmdline "init=/init" --network user-vsock --gvproxy C:\\path\\to\\gvproxy.exe --gvproxy-vsock-port $gvproxy_vsock_port

After stopping the VM, inspect hcs-test-data.vhd and look for /usernet-ok.txt.
EOF_DONE
else
  cat <<EOF_DONE

Windows smoke-test command:
  dotnet run -- --root C:\\vm\\hcs-test-root.vhd --data C:\\vm\\hcs-test-data.vhd --root-device /dev/sda --no-initrd --append-kernel-cmdline "init=/init" --network none

After stopping the VM, mount or inspect hcs-test-data.vhd and look for /boot-ok.txt.
EOF_DONE
fi
