distro_python_sitelib=$(/usr/libexec/platform-python -c "from distutils.sysconfig import get_python_lib; print(get_python_lib())")
if [ $1 -eq 0 ]; then
    semodule -n -r receptor
    if /usr/sbin/selinuxenabled ; then
       /usr/sbin/load_policy
    fi;
    rm -f ${distro_python_sitelib}/sos/report/plugins/receptor.py || :
fi;
exit 0
