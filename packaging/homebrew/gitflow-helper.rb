# Homebrew Formula for gitflow-helper
# To install from tap: brew install <org>/tap/gitflow-helper
# To use locally: brew install --formula ./packaging/homebrew/gitflow-helper.rb
class GitflowHelper < Formula
  desc "Git Flow workflow helper — interactive TUI + CLI. Only requires git."
  homepage "https://github.com/novaemx/gitflow-helper"
  version "0.5.35"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/novaemx/gitflow-helper/releases/download/v0.5.35/gitflow-0.5.35-darwin-universal.tar.gz"
      sha256 "b7ea0b9ae5c5f598a3378e2a9621dbd9ba96ee582b26fbf3a1500b5d65a3cc6c"
    else
      url "https://github.com/novaemx/gitflow-helper/releases/download/v0.5.35/gitflow-0.5.35-darwin-universal.tar.gz"
      sha256 "b7ea0b9ae5c5f598a3378e2a9621dbd9ba96ee582b26fbf3a1500b5d65a3cc6c"
    end
  end

  depends_on "git"

  def install
    bin.install "gitflow"
  end

  test do
    assert_match "gitflow", shell_output("#{bin}/gitflow --version")
  end
end
