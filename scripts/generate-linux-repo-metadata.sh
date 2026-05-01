#!/usr/bin/env bash

set -euo pipefail

usage() {
	cat <<'EOF'
Usage: generate-linux-repo-metadata.sh --version <version> --repo <owner/name> --dist <dir> [options]

Options:
  --apt-assets-dir <dir>    Generate APT flat-repo assets (Packages, Packages.gz, Release, .sources)
  --apt-source-file <file>  Write the Debian/Ubuntu .sources file to a tracked location
  --yum-repo-file <file>    Write the Rocky Linux .repo file
  --yum-root <dir>          Generate YUM repodata under <dir>/<arch>/repodata
  --arch-root <dir>         Generate pacman DB files under <dir>/{x86_64,aarch64}/
  --arch-pkgbuild <file>    Update a tracked PKGBUILD with the current version and checksums
  --arch-conf-file <file>   Write the Arch/CachyOS pacman .conf snippet to a tracked location
EOF
}

version=""
repo=""
dist_dir=""
apt_assets_dir=""
apt_source_file=""
yum_repo_file=""
yum_root=""
arch_root=""
arch_pkgbuild=""
arch_conf_file=""

while [ $# -gt 0 ]; do
	case "$1" in
		--version)
			version="$2"
			shift 2
			;;
		--repo)
			repo="$2"
			shift 2
			;;
		--dist)
			dist_dir="$2"
			shift 2
			;;
		--apt-assets-dir)
			apt_assets_dir="$2"
			shift 2
			;;
		--apt-source-file)
			apt_source_file="$2"
			shift 2
			;;
		--yum-repo-file)
			yum_repo_file="$2"
			shift 2
			;;
		--yum-root)
			yum_root="$2"
			shift 2
			;;
		--arch-root)
			arch_root="$2"
			shift 2
			;;
		--arch-pkgbuild)
			arch_pkgbuild="$2"
			shift 2
			;;
		--arch-conf-file)
			arch_conf_file="$2"
			shift 2
			;;
		-h|--help)
			usage
			exit 0
			;;
		*)
			echo "Unknown argument: $1" >&2
			usage >&2
			exit 1
			;;
	esac
done

[ -n "$version" ] || { echo "--version is required" >&2; exit 1; }
[ -n "$repo" ] || { echo "--repo is required" >&2; exit 1; }
[ -n "$dist_dir" ] || { echo "--dist is required" >&2; exit 1; }

script_dir=$(cd "$(dirname "$0")" && pwd)
root_dir=$(cd "$script_dir/.." && pwd)
license_file="$root_dir/LICENSE"

[ -f "$license_file" ] || { echo "Missing LICENSE file" >&2; exit 1; }

release_base="https://github.com/${repo}/releases/latest/download"
rocky_base="https://raw.githubusercontent.com/${repo}/main/packaging/linux/yum/rocky/9"
package_name="gitflow-helper"
description="Git Flow workflow helper with interactive TUI and CLI."
maintainer="Luis Lozano"
homepage="https://github.com/${repo}"

license_size=$(stat -c '%s' "$license_file")
timestamp=$(date +%s)
rfc2822_date=$(date -Ru)

deb_file_for_arch() {
	case "$1" in
		amd64) echo "${dist_dir}/${package_name}_${version}_linux_amd64.deb" ;;
		arm64) echo "${dist_dir}/${package_name}_${version}_linux_arm64.deb" ;;
		*) return 1 ;;
	esac
}

rpm_file_for_arch() {
	case "$1" in
		x86_64) echo "${dist_dir}/${package_name}_${version}_linux_amd64.rpm" ;;
		aarch64) echo "${dist_dir}/${package_name}_${version}_linux_arm64.rpm" ;;
		*) return 1 ;;
	esac
}

binary_file_for_arch() {
	case "$1" in
		amd64|x86_64) echo "${dist_dir}/linux-amd64_linux_amd64_v1/gitflow" ;;
		arm64|aarch64) echo "${dist_dir}/linux-arm64_linux_arm64_v8.0/gitflow" ;;
		*) return 1 ;;
	esac
}

pkg_file_for_arch() {
	# Returns the .pkg.tar.zst path for the given pacman arch (x86_64 or aarch64).
	case "$1" in
		x86_64)  echo "${dist_dir}/${package_name}_${version}_linux_amd64.pkg.tar.zst" ;;
		aarch64) echo "${dist_dir}/${package_name}_${version}_linux_arm64.pkg.tar.zst" ;;
		*) return 1 ;;
	esac
}

write_apt_source() {
	local target_file="$1"
	mkdir -p "$(dirname "$target_file")"
	cat > "$target_file" <<EOF
Types: deb
URIs: ${release_base}/
Suites: ./
Architectures: amd64 arm64
Trusted: yes
EOF
}

generate_apt_assets() {
	local out_dir="$1"
	local packages_file="$out_dir/Packages"
	local packages_gz="$out_dir/Packages.gz"
	local release_file="$out_dir/Release"
	local source_file="$out_dir/gitflow-helper.sources"
	local package_arch

	mkdir -p "$out_dir"
	: > "$packages_file"

	for package_arch in amd64 arm64; do
		local deb_file deb_name deb_size deb_sha deb_md5 binary_file installed_size
		deb_file=$(deb_file_for_arch "$package_arch")
		binary_file=$(binary_file_for_arch "$package_arch")
		[ -f "$deb_file" ] || { echo "Missing package: $deb_file" >&2; exit 1; }
		[ -f "$binary_file" ] || { echo "Missing binary: $binary_file" >&2; exit 1; }
		deb_name=$(basename "$deb_file")
		deb_size=$(stat -c '%s' "$deb_file")
		deb_sha=$(sha256sum "$deb_file" | awk '{print $1}')
		deb_md5=$(md5sum "$deb_file" | awk '{print $1}')
		installed_size=$(( ( $(stat -c '%s' "$binary_file") + license_size + 1023 ) / 1024 ))
		cat >> "$packages_file" <<EOF
Package: ${package_name}
Version: ${version}
Architecture: ${package_arch}
Maintainer: ${maintainer}
Installed-Size: ${installed_size}
Section: utils
Priority: optional
Homepage: ${homepage}
Filename: ${deb_name}
Size: ${deb_size}
MD5sum: ${deb_md5}
SHA256: ${deb_sha}
Description: ${description}

EOF
	done

	gzip -n -9 -c "$packages_file" > "$packages_gz"
	local packages_size packages_gz_size packages_md5 packages_gz_md5 packages_sha packages_gz_sha
	packages_size=$(stat -c '%s' "$packages_file")
	packages_gz_size=$(stat -c '%s' "$packages_gz")
	packages_md5=$(md5sum "$packages_file" | awk '{print $1}')
	packages_gz_md5=$(md5sum "$packages_gz" | awk '{print $1}')
	packages_sha=$(sha256sum "$packages_file" | awk '{print $1}')
	packages_gz_sha=$(sha256sum "$packages_gz" | awk '{print $1}')
	cat > "$release_file" <<EOF
Origin: NovaeMX
Label: gitflow-helper
Suite: stable
Codename: stable
Version: ${version}
Architectures: amd64 arm64
Components: main
Date: ${rfc2822_date}
MD5Sum:
 ${packages_md5} ${packages_size} Packages
 ${packages_gz_md5} ${packages_gz_size} Packages.gz
SHA256:
 ${packages_sha} ${packages_size} Packages
 ${packages_gz_sha} ${packages_gz_size} Packages.gz
EOF
	write_apt_source "$source_file"
}

write_yum_repo_file() {
	local target_file="$1"
	mkdir -p "$(dirname "$target_file")"
	cat > "$target_file" <<EOF
[gitflow-helper]
name=gitflow-helper
baseurl=${rocky_base}/\$basearch
enabled=1
gpgcheck=0
repo_gpgcheck=0
metadata_expire=300
skip_if_unavailable=False
EOF
}

generate_yum_arch() {
	local yum_arch="$1"
	local arch_dir="$2/$yum_arch/repodata"
	local rpm_file rpm_name rpm_size rpm_sha binary_file installed_bytes primary filelists other repomd
	rpm_file=$(rpm_file_for_arch "$yum_arch")
	binary_file=$(binary_file_for_arch "$yum_arch")
	[ -f "$rpm_file" ] || { echo "Missing package: $rpm_file" >&2; exit 1; }
	[ -f "$binary_file" ] || { echo "Missing binary: $binary_file" >&2; exit 1; }
	rpm_name=$(basename "$rpm_file")
	rpm_size=$(stat -c '%s' "$rpm_file")
	rpm_sha=$(sha256sum "$rpm_file" | awk '{print $1}')
	installed_bytes=$(( $(stat -c '%s' "$binary_file") + license_size ))
	primary="$arch_dir/primary.xml"
	filelists="$arch_dir/filelists.xml"
	other="$arch_dir/other.xml"
	repomd="$arch_dir/repomd.xml"
	mkdir -p "$arch_dir"
	cat > "$primary" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<metadata xmlns="http://linux.duke.edu/metadata/common" xmlns:rpm="http://linux.duke.edu/metadata/rpm" packages="1">
  <package type="rpm">
    <name>${package_name}</name>
    <arch>${yum_arch}</arch>
    <version epoch="0" ver="${version}" rel="1"/>
    <checksum type="sha256" pkgid="YES">${rpm_sha}</checksum>
    <summary>${description}</summary>
    <description>${description}</description>
    <packager>${maintainer}</packager>
    <url>${homepage}</url>
    <time file="${timestamp}" build="${timestamp}"/>
    <size package="${rpm_size}" installed="${installed_bytes}" archive="${rpm_size}"/>
    <location xml:base="${release_base}/" href="${rpm_name}"/>
    <format>
      <rpm:license>MIT</rpm:license>
      <rpm:vendor>NovaeMX</rpm:vendor>
      <rpm:group>Applications/Utilities</rpm:group>
      <rpm:buildhost>github.com</rpm:buildhost>
      <rpm:header-range start="0" end="0"/>
      <rpm:provides>
        <rpm:entry name="${package_name}" flags="EQ" epoch="0" ver="${version}" rel="1"/>
      </rpm:provides>
    </format>
  </package>
</metadata>
EOF
	cat > "$filelists" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<filelists xmlns="http://linux.duke.edu/metadata/filelists" packages="1">
  <package pkgid="${rpm_sha}" name="${package_name}" arch="${yum_arch}">
    <version epoch="0" ver="${version}" rel="1"/>
    <file>/usr/bin/gitflow</file>
    <file>/usr/share/licenses/gitflow-helper/LICENSE</file>
  </package>
</filelists>
EOF
	cat > "$other" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<otherdata xmlns="http://linux.duke.edu/metadata/other" packages="1">
  <package pkgid="${rpm_sha}" name="${package_name}" arch="${yum_arch}">
    <version epoch="0" ver="${version}" rel="1"/>
    <changelog author="${maintainer}" date="${timestamp}">GitHub release ${version}</changelog>
  </package>
</otherdata>
EOF
	local primary_sha primary_size filelists_sha filelists_size other_sha other_size
	primary_sha=$(sha256sum "$primary" | awk '{print $1}')
	primary_size=$(stat -c '%s' "$primary")
	filelists_sha=$(sha256sum "$filelists" | awk '{print $1}')
	filelists_size=$(stat -c '%s' "$filelists")
	other_sha=$(sha256sum "$other" | awk '{print $1}')
	other_size=$(stat -c '%s' "$other")
	cat > "$repomd" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<repomd xmlns="http://linux.duke.edu/metadata/repo">
  <revision>${timestamp}</revision>
  <data type="primary">
    <checksum type="sha256">${primary_sha}</checksum>
    <open-checksum type="sha256">${primary_sha}</open-checksum>
    <location href="repodata/primary.xml"/>
    <timestamp>${timestamp}</timestamp>
    <size>${primary_size}</size>
    <open-size>${primary_size}</open-size>
  </data>
  <data type="filelists">
    <checksum type="sha256">${filelists_sha}</checksum>
    <open-checksum type="sha256">${filelists_sha}</open-checksum>
    <location href="repodata/filelists.xml"/>
    <timestamp>${timestamp}</timestamp>
    <size>${filelists_size}</size>
    <open-size>${filelists_size}</open-size>
  </data>
  <data type="other">
    <checksum type="sha256">${other_sha}</checksum>
    <open-checksum type="sha256">${other_sha}</open-checksum>
    <location href="repodata/other.xml"/>
    <timestamp>${timestamp}</timestamp>
    <size>${other_size}</size>
    <open-size>${other_size}</open-size>
  </data>
</repomd>
EOF
}

if [ -n "$apt_assets_dir" ]; then
	generate_apt_assets "$apt_assets_dir"
fi

if [ -n "$apt_source_file" ]; then
	write_apt_source "$apt_source_file"
fi

if [ -n "$yum_repo_file" ]; then
	write_yum_repo_file "$yum_repo_file"
fi

if [ -n "$yum_root" ]; then
	generate_yum_arch x86_64 "$yum_root"
	generate_yum_arch aarch64 "$yum_root"
fi

# ── Arch Linux / CachyOS support ────────────────────────────────────────────

# update_arch_pkgbuild updates pkgver and sha256sums in a tracked PKGBUILD.
update_arch_pkgbuild() {
	local pkgbuild_file="$1"
	local amd64_sha aarch64_sha amd64_file aarch64_file
	amd64_file=$(pkg_file_for_arch x86_64)
	aarch64_file=$(pkg_file_for_arch aarch64)
	[ -f "$amd64_file" ]   || { echo "Missing Arch package: $amd64_file" >&2; exit 1; }
	[ -f "$aarch64_file" ] || { echo "Missing Arch package: $aarch64_file" >&2; exit 1; }
	amd64_sha=$(sha256sum "$amd64_file" | awk '{print $1}')
	aarch64_sha=$(sha256sum "$aarch64_file" | awk '{print $1}')
	sed \
		-e "s|^pkgver=.*|pkgver=${version}|" \
		-e "s|/releases/download/v[^/]*/gitflow-helper_[^/]*_linux_amd64.pkg.tar.zst|/releases/download/v${version}/gitflow-helper_${version}_linux_amd64.pkg.tar.zst|" \
		-e "s|/releases/download/v[^/]*/gitflow-helper_[^/]*_linux_arm64.pkg.tar.zst|/releases/download/v${version}/gitflow-helper_${version}_linux_arm64.pkg.tar.zst|" \
		"$pkgbuild_file" > "${pkgbuild_file}.tmp"
	# Replace sha256sums_x86_64 and sha256sums_aarch64 placeholder or existing value.
	awk \
		-v amd64_sha="$amd64_sha" \
		-v aarch64_sha="$aarch64_sha" \
		"
		/^sha256sums_x86_64=/ { print \"sha256sums_x86_64=(\" \"'\" amd64_sha \"'\" \")\"; next }
		/^sha256sums_aarch64=/ { print \"sha256sums_aarch64=(\" \"'\" aarch64_sha \"'\" \")\"; next }
		{ print }
		" "${pkgbuild_file}.tmp" > "${pkgbuild_file}.tmp2"
	mv "${pkgbuild_file}.tmp2" "$pkgbuild_file"
	rm -f "${pkgbuild_file}.tmp"
}

# generate_arch_db creates a minimal pacman custom repo database (no repo-add needed).
# The database is a gzipped tar archive with one entry per package containing a
# "desc" file (standard pacman format). It is placed at <arch_root>/<arch>/
generate_arch_db() {
	local pacman_arch="$1"
	local arch_dir="$2/$pacman_arch"
	local pkg_file pkg_name pkg_size pkg_sha binary_file installed_bytes
	pkg_file=$(pkg_file_for_arch "$pacman_arch")
	binary_file=$(binary_file_for_arch "$pacman_arch")
	[ -f "$pkg_file" ]    || { echo "Missing Arch package: $pkg_file" >&2; exit 1; }
	[ -f "$binary_file" ] || { echo "Missing binary: $binary_file" >&2; exit 1; }
	pkg_name=$(basename "$pkg_file")
	pkg_size=$(stat -c '%s' "$pkg_file")
	pkg_sha=$(sha256sum "$pkg_file" | awk '{print $1}')
	installed_bytes=$(( $(stat -c '%s' "$binary_file") + license_size ))
	mkdir -p "$arch_dir"

	# Build pacman desc in a temp dir then tar it.
	local tmp_db
	tmp_db=$(mktemp -d)
	local entry_dir="${tmp_db}/${package_name}-${version}-1"
	mkdir -p "$entry_dir"

	cat > "${entry_dir}/desc" <<EOF
%FILENAME%
${pkg_name}

%NAME%
${package_name}

%BASE%
${package_name}

%VERSION%
${version}-1

%DESC%
${description}

%CSIZE%
${pkg_size}

%ISIZE%
${installed_bytes}

%SHA256SUM%
${pkg_sha}

%URL%
${homepage}

%LICENSE%
MIT

%ARCH%
${pacman_arch}

%BUILDDATE%
${timestamp}

%PACKAGER%
${maintainer}

EOF

	cat > "${entry_dir}/files" <<EOF
%FILES%
usr/
usr/bin/
usr/bin/gitflow
usr/share/
usr/share/licenses/
usr/share/licenses/${package_name}/
usr/share/licenses/${package_name}/LICENSE

EOF

	# Create the .db.tar.gz and a .db symlink (pacman convention).
	local db_tar="${arch_dir}/${package_name}.db.tar.gz"
	local db_link="${arch_dir}/${package_name}.db"
	tar -czf "$db_tar" -C "$tmp_db" .
	cp -f "$db_tar" "$db_link"   # copy instead of symlink for cross-platform compatibility
	rm -rf "$tmp_db"
}

write_arch_conf() {
	local target_file="$1"
	mkdir -p "$(dirname "$target_file")"
	cat > "$target_file" <<EOF
# gitflow-helper pacman custom repository
# For Arch Linux, CachyOS, EndeavourOS, Garuda, and other Arch-based distros.
#
# Add this repository to /etc/pacman.conf:
#
#   [gitflow-helper]
#   Server = https://github.com/novaemx/gitflow-helper/releases/latest/download
#   SigLevel = Optional TrustAll
#
# Or use the AUR helper approach (recommended):
#   yay -S gitflow-helper
#   paru -S gitflow-helper
#
# --- pacman.conf snippet (copy/paste) ---
[gitflow-helper]
Server = https://github.com/novaemx/gitflow-helper/releases/latest/download
SigLevel = Optional TrustAll
EOF
}

if [ -n "$arch_pkgbuild" ]; then
	update_arch_pkgbuild "$arch_pkgbuild"
fi

if [ -n "$arch_conf_file" ]; then
	write_arch_conf "$arch_conf_file"
fi

if [ -n "$arch_root" ]; then
	generate_arch_db x86_64 "$arch_root"
	generate_arch_db aarch64 "$arch_root"
fi