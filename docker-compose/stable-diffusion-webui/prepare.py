from modules import launch_utils


def main():
    if not launch_utils.args.skip_prepare_environment:
        launch_utils.prepare_environment()

    if launch_utils.args.test_server:
        launch_utils.configure_for_tests()


if __name__ == "__main__":
    main()
