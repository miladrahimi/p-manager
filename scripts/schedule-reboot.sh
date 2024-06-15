# Setup cron job for reboot
COMMAND="reboot"
if ! crontab -l | grep -q "$COMMAND"; then
    (crontab -l 2>/dev/null; echo "0 5 * * * $COMMAND") | crontab -
    echo "The updater cron job configured."
else
    echo "The updater cron job is already configured."
fi