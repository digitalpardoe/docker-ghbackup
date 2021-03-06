#!/usr/bin/env ruby

require 'octokit'
require 'uri'

lock_file = File.open("/tmp/ghbackup.lock", File::CREAT)
lock_state = lock_file.flock(File::LOCK_EX|File::LOCK_NB)

if !lock_state
  puts "Already running, exiting..."
  exit
end

begin
  Octokit.configure do |c|
    c.auto_paginate = true
  end

  github_secret = ENV["GITHUB_SECRET"]
  backup_folder = ENV["BACKUP_FOLDER"] || "/ghbackup"

  client = Octokit::Client.new(access_token: github_secret)

  login = client.user[:login]

  client.repos.each do |repo|
    uri = URI.parse(repo[:clone_url])
    authenitcated_clone_url = "#{uri.scheme}://#{login}:#{github_secret}@#{uri.host}#{uri.path}"
    
    backup_path = "#{backup_folder}/#{repo[:full_name]}.git"

    p "Backing up #{repo[:full_name]}..."

    if Dir.exist?(backup_path)
      Dir.chdir(backup_path) {
        system('git', 'remote', 'update')
      }
    else
      system('git', 'clone', '--mirror', '--no-checkout', '--progress', authenitcated_clone_url, backup_path)
    end
  end
ensure
  lock_file.close
end