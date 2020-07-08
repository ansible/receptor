BYTES_PAYLOAD = "bytes"
"""
Inform Receptor that the given plugin expects BYTES for the message data
"""

BUFFER_PAYLOAD = "buffer"
"""
Inform Receptor that the given plugin expects a buffered reader for the message data
"""

FILE_PAYLOAD = "file"
"""
Inform Receptor that the given plugin expects a file path for the message data
"""


def plugin_export(payload_type):
    """
    A decorator intended to be used by Receptor plugins in conjunction with
    entrypoints typically defined in your setup.py file::

        entry_points={
            'receptor.worker':
              'your_package_name = your_package_name.your_module',
        }

    ``your_package_name.your_module`` should then contain a function decorated with
    ``plugin_export`` as such::

        @receptor.plugin_export(payload_type=receptor.BYTES_PAYLOAD):
        def execute(message, config, result_queue):
            result_queue.put("My plugin ran!")

    You can then send messages to this plugin across the Receptor mesh with the directive
    ``your_package_name:execute``

    Depending on what kind of data you expect to receive you can select from one
    of 3 different incoming payload types. This determines the incoming type of the
    ``message`` data type:

    * BYTES_PAYLOAD: This will give you literal python bytes that you can then read
      and interpret.
    * BUFFER_PAYLOAD: This will send you a buffer that you can read(). This buffer
      will be automatically closed and its contents discarded when your plugin returns.
    * FILE_PAYLOAD: This will return you a file path that you can open() or manage
      in any way you see fit. It will be automatically removed after your plugin returns.

    For more information about developing plugins see :ref:`plugins`.
    """

    def decorator(func):
        func.receptor_export = True
        func.payload_type = payload_type
        return func

    return decorator
