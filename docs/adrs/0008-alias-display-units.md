# Alias display units

An alias rule may specify a positive integer `unit`, the number of underlying resource units represented by one displayed alias unit. Matching Node allocatable and Running Pod requests are divided by this value when attributed to the alias; Pending Pods remain unscaled under their raw resource name.

`unit` defaults to `1` for existing rules. The same scaling applies to capacity and requests, preserving utilisation ratios. Alias quantities retain milli-unit precision when division is not whole.
