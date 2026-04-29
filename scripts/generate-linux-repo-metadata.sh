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
EOF
}

version=""
repo=""
dist_dir=""
apt_assets_dir=""
apt_source_file=""
yum_repo_file=""
yum_root=""

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