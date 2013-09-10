def test_dir(dir)
    puts ""
    puts "~"*80
    test_status = system("pushd #{dir} > /dev/null && go test -i && go test --ginkgo.randomizeAllSpecs && popd")
    if !test_status
        puts ""
        puts "!!!!!!!!!!! TESTS FAILED: #{dir} !!!!!!!!!!!!!"
        exit(1)
    end
end

desc "Run all the test_helper tests"
task :test_helpers do |t|
    test_dir("./test_helpers/app")
    test_dir("./test_helpers/message_publisher")
    test_dir("./test_helpers/desired_state_server")
end

desc "Run all the component tests"
task :test_components do |t|
    test_dir("./actualstatelistener")
    test_dir("./store")
end

task :default => [:test_helpers, :test_components]