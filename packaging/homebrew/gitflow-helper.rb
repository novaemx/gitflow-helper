# Homebrew Formula for gitflow-helper
# To install from tap: brew install <org>/tap/gitflow-helper
# To use locally: brew install --formula ./packaging/homebrew/gitflow-helper.rb
class GitflowHelper < Formula
  desc "Git Flow workflow helper — interactive TUI + CLI. Only requires git."
  homepage "https://github.com/novaemx/gitflow-helper"
  version "0.5.38"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/novaemx/gitflow-helper/releases/download/v0.5.38/gitflow-0.5.38-darwin-universal.tar.gz"
      sha256 "42364615724999bd3c2b20495011956bc4a2fd15d193702dfbaf12a07db08c88"
    else
      url "https://github.com/novaemx/gitflow-helper/releases/download/v0.5.38/gitflow-0.5.38-darwin-universal.tar.gz"
      sha256 "42364615724999bd3c2b20495011956bc4a2fd15d193702dfbaf12a07db08c88"
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
