import {useState, useCallback} from "https://cdn.skypack.dev/preact@10.22.1/hooks";

const useNotification = () => {
    const [notifications, setNotifications] = useState({
        success: null,
        error: null
    });

    const showNotification = useCallback((type, message, duration = 5000) => {
        setNotifications(prev => ({
            ...prev,
            [type]: message
        }));

        setTimeout(() => {
            setNotifications(prev => ({
                ...prev,
                [type]: null
            }));
        }, duration);
    }, []);

    const showSuccess = useCallback((message, duration) => {
        showNotification('success', message, duration);
    }, [showNotification]);

    const showError = useCallback((message, duration) => {
        showNotification('error', message, duration);
    }, [showNotification]);

    const clearNotification = useCallback((type) => {
        if (type) {
            setNotifications(prev => ({
                ...prev,
                [type]: null
            }));
        } else {
            setNotifications({
                success: null,
                error: null
            });
        }
    }, []);

    return {
        notifications,
        showSuccess,
        showError,
        clearNotification
    };
};

export default useNotification;