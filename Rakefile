def test_dir(dir)
    puts "~~~~~~~~~~~~~~~~~~~~~~~"
    puts "Running tests in #{dir}"
    puts ""
    test_status = system("pushd #{dir} && go test -i && go test --ginkgo.randomizeAllSpecs && popd")
    if !test_status
        puts ""
        puts "!!!!!!!!!!! TESTS FAILED: #{dir} !!!!!!!!!!!!!"
        exit(1)
    end
end

desc "Run all the component tests"
task :test do |t|
    test_dir("./test_helpers/message_publisher")
    test_dir("./test_helpers/app")
    test_dir("./actual_state_listener")
    test_dir("./store")
end

task :default => :test