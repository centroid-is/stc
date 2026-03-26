{IF defined(VENDOR_BECKHOFF)}
    (* Beckhoff OK *)
{ELSIF defined(VENDOR_SCHNEIDER)}
    (* Schneider OK *)
{ELSE}
    {ERROR "No supported vendor defined. Use --define VENDOR_BECKHOFF or VENDOR_SCHNEIDER"}
{END_IF}
