# Homebrew Formula for gitflow-helper
# To install from tap: brew install <org>/tap/gitflow-helper
# To use locally: brew install --formula ./packaging/homebrew/gitflow-helper.rb
class GitflowHelper < Formula
  desc "Git Flow workflow helper — interactive TUI + CLI. Only requires git."
  homepage "https://github.com/novaemx/gitflow-helper"
  version "0.5.39"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/novaemx/gitflow-helper/releases/download/v0.5.39/gitflow-0.5.39-darwin-universal.tar.gz"
      sha256 "532fbedc51c783389447fda28e1256074d4408dc0b6349205143e1a1ac01469e"
    else
      url "https://github.com/novaemx/gitflow-helper/releases/download/v0.5.39/gitflow-0.5.39-darwin-universal.tar.gz"
      sha256 "532fbedc51c783389447fda28e1256074d4408dc0b6349205143e1a1ac01469e"
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
