import modules.launch_utils


def main():
    if not modules.launch_utils.args.skip_prepare_environment:
        modules.launch_utils.prepare_environment()

    if modules.launch_utils.args.test_server:
        modules.launch_utils.configure_for_tests()


if __name__ == "__main__":
    main()
