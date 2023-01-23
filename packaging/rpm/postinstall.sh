distro_python_sitelib=$(/usr/libexec/platform-python -c "from distutils.sysconfig import get_python_lib; print(get_python_lib())")
semodule -n -i /usr/share/selinux/packages/receptor.pp
if /usr/sbin/selinuxenabled ; then
    /usr/sbin/load_policy
fi;
ln -s /usr/share/sosreport/sos/plugins/receptor.py ${distro_python_sitelib}/sos/report/plugins/receptor.py || :
exit 0
