class Gitflow < Formula
  desc "Git Flow workflow helper with interactive TUI and CLI"
  homepage "https://github.com/novaemx/gitflow-helper"
  url "https://github.com/novaemx/gitflow-helper/releases/download/v0.7.0/gitflow-0.7.0-darwin-arm64.tar.gz"
  version "0.7.0"
  sha256 "efb62b6281d0d55c1033bee87234a5c452ef2517f492f0a3d3b27a684626645f"
  license "MIT"

  def install
    bin.install "gitflow"
  end

  test do
    output = shell_output("#{bin}/gitflow --version")
    assert_match version.to_s, output
  end
end
