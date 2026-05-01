class Gitflow < Formula
  desc "Git Flow workflow helper with interactive TUI and CLI"
  homepage "https://github.com/novaemx/gitflow-helper"
  url "https://github.com/novaemx/gitflow-helper/releases/download/v0.5.51/gitflow-0.5.51-darwin-universal.tar.gz"
  version "0.5.51"
  sha256 "aed4e039d02a3c05329c76a2823c415910b2448628af8a45597a8a8c1310adcb"
  license "MIT"

  def install
    bin.install "gitflow"
  end

  test do
    output = shell_output("#{bin}/gitflow --version")
    assert_match version.to_s, output
  end
end