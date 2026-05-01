class Gitflow < Formula
  desc "Git Flow workflow helper with interactive TUI and CLI"
  homepage "https://github.com/novaemx/gitflow-helper"
  url "https://github.com/novaemx/gitflow-helper/releases/download/v0.6.0/gitflow-0.6.0-darwin-universal.tar.gz"
  version "0.6.0"
  sha256 "0dce791d0b700e36a63637efccf3eb3112fe67a057a1a774ef668f1a60446a8a"
  license "MIT"

  def install
    bin.install "gitflow"
  end

  test do
    output = shell_output("#{bin}/gitflow --version")
    assert_match version.to_s, output
  end
end
