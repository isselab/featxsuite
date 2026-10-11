# Draft comment for ros2/rclpy#575

Not posted. Review and edit before sending; it will appear under your name.

---

I have a working implementation of this, and would like to know which
registration mechanism you would prefer before cleaning it up for review.

**What it does.** `rclpy_components` provides `component_container` and
`component_container_mt`, which serve the three `composition_interfaces`
services (`LoadNode`, `UnloadNode`, `ListNodes`) under the usual
`~/_container/*` names. Loading imports the registered module, instantiates the
class, and adds the node to the container's executor so its callbacks run;
unloading removes it and destroys the node. The stock CLI drives it unchanged:

```
ros2 run rclpy_components component_container_mt
ros2 component load /rclpy_container my_package my_ns::MyNode
ros2 component list
ros2 component unload /rclpy_container 1
```

Tested over DDS against a separate container process, and with 26 unit tests.

**The question.** This thread proposes registering components through setuptools
entry points in an `rclpy_components` group. I used an ament resource index
entry instead, with the same format `rclcpp_components` uses — one
`name;target` line per component, where the target is `module:Class` rather
than a library path:

```
share/ament_index/resource_index/rclpy_components/<package>
  my_ns::MyNode;my_package.my_node:MyNode
```

My reason for preferring it is tooling uniformity: because the file format
matches, `ros2component` can support both languages by searching both groups
rather than growing a second discovery path. At present
`ros2component/api/__init__.py` hardcodes `COMPONENTS_RESOURCE_TYPE =
'rclcpp_components'`, so `ros2 component types` cannot see Python components
even though `load`, `unload` and `list` work fine — those go through services
and do not care about the language. With a shared format that is close to a
one-line change; with entry points it is a separate code path.

I am happy to switch to entry points if you would rather keep Python
registration Pythonic. It is the one decision that changes the shape of the PR,
which is why I am asking first.

**Two things that only came out of implementing it**, in case they are useful
regardless of which way this goes:

1. Components have to be constructed with `use_global_arguments=False`.
   Without it, a container started with `-r __node:=x` renames every component
   it loads, because rclpy applies global arguments to every node created in
   the process.

2. A component class with no `__init__` of its own inherits `object.__init__`,
   whose signature is `(*args, **kwargs)`, so it looks like it accepts node
   options. Such a class has to be detected and constructed bare, and options
   sent for it refused rather than silently dropped.

**Not implemented:** the service reporting which component types a container
supports, suggested earlier in this thread. That is not in
`composition_interfaces`, so it would need to be added there; I left it out of
scope.

**One limitation worth stating plainly.** Unloading cannot reclaim code. Python
has no `dlclose`, and dropping the module from `sys.modules` only clears the
import cache, so a stopped component's module stays resident. The node is
destroyed and its callbacks stop, which is what is observable over ROS, but the
memory is not returned.

Code: <your branch URL>

---

## Checklist before the PR

- [ ] Apache-2.0 header on every source file (`ament_copyright` enforces it)
- [ ] `test_copyright.py`, `test_flake8.py`, `test_pep257.py`
- [ ] `ament_flake8` and `ament_pep257` clean
- [ ] Real name and contact in `package.xml` as maintainer
- [ ] Decide the registration mechanism from the answer to the question above
- [ ] Companion change to `ros2/ros2cli` so `ros2component` searches both groups
- [ ] Drop `examples.py`, or move it to a test fixture — it exists for the tests
