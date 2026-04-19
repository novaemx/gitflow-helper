# Homebrew Formula for gitflow-helper
# To install from tap: brew install <org>/tap/gitflow-helper
# To use locally: brew install --formula ./packaging/homebrew/gitflow-helper.rb
class GitflowHelper < Formula
  desc "Git Flow workflow helper — interactive TUI + CLI. Only requires git."
  homepage "https://github.com/novaemx/gitflow-helper"
  version "0.5.34"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/novaemx/gitflow-helper/releases/download/v0.5.34/gitflow-0.5.34-darwin-universal.tar.gz"
      sha256 "61dade8360e57a349ee937b354dabad3fd165f0d087c2d886c46d665b64dcbf2"
    else
      url "https://github.com/novaemx/gitflow-helper/releases/download/v0.5.34/gitflow-0.5.34-darwin-universal.tar.gz"
      sha256 "61dade8360e57a349ee937b354dabad3fd165f0d087c2d886c46d665b64dcbf2"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/novaemx/gitflow-helper/releases/download/v0.5.34/gitflow-0.5.34-linux-amd64.tar.gz"
      sha256 "c7fa6befaf79698c20822f7e20f5da092e3e057303a9a9653d6a06d7da6e6313"
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
