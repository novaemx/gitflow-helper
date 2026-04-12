# Homebrew Formula for gitflow-helper
# To install from tap: brew install <org>/tap/gitflow-helper
# To use locally: brew install --formula ./packaging/homebrew/gitflow-helper.rb
class GitflowHelper < Formula
  desc "Git Flow workflow helper — interactive TUI + CLI. Only requires git."
  homepage "https://github.com/luis-lozano/gitflow-helper"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/luis-lozano/gitflow-helper/releases/download/v#{version}/gitflow-#{version}-darwin-universal.tar.gz"
      sha256 "PLACEHOLDER_SHA256_DARWIN"
    else
      url "https://github.com/luis-lozano/gitflow-helper/releases/download/v#{version}/gitflow-#{version}-darwin-universal.tar.gz"
      sha256 "PLACEHOLDER_SHA256_DARWIN"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/luis-lozano/gitflow-helper/releases/download/v#{version}/gitflow-#{version}-linux-amd64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_LINUX"
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
