# Homebrew Formula for gitflow-helper
# To install from tap: brew install <org>/tap/gitflow-helper
# To use locally: brew install --formula ./packaging/homebrew/gitflow-helper.rb
class GitflowHelper < Formula
  desc "Git Flow workflow helper — interactive TUI + CLI. Only requires git."
  homepage "https://github.com/novaemx/gitflow-helper"
  version "0.5.40"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/novaemx/gitflow-helper/releases/download/v0.5.40/gitflow-0.5.40-darwin-universal.tar.gz"
      sha256 "1b8b62f54590cba5d5f9a3ccf0de043a5c48e3f95f74919a2023ba37d0d120cd"
    else
      url "https://github.com/novaemx/gitflow-helper/releases/download/v0.5.40/gitflow-0.5.40-darwin-universal.tar.gz"
      sha256 "1b8b62f54590cba5d5f9a3ccf0de043a5c48e3f95f74919a2023ba37d0d120cd"
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
